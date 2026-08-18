// Command cachekey fingerprints a Go main package for build-go binary caching.
package main

import (
	"archive/tar"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
)

type goPackage struct {
	Dir        string   `json:"Dir"`
	Standard   bool     `json:"Standard"`
	GoFiles    []string `json:"GoFiles"`
	EmbedFiles []string `json:"EmbedFiles"`
}

func main() {
	mainPackage := flag.String("main-package", "", "main package path (e.g. ./cmd/server)")
	flag.Parse()

	if *mainPackage == "" {
		fmt.Fprintln(os.Stderr, "cachekey: --main-package is required")
		os.Exit(2)
	}

	key, err := fingerprint(*mainPackage)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cachekey: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(key)
}

func fingerprint(mainPackage string) (string, error) {
	paths, err := sourcePaths(mainPackage)
	if err != nil {
		return "", err
	}

	sum, err := hashPaths(paths)
	if err != nil {
		return "", err
	}

	return hex.EncodeToString(sum), nil
}

func sourcePaths(mainPackage string) ([]string, error) {
	pathSet := make(map[string]struct{})

	for _, moduleFile := range []string{"go.mod", "go.sum"} {
		if _, err := os.Stat(moduleFile); err == nil {
			pathSet[moduleFile] = struct{}{}
		}
	}

	cmd := exec.Command("go", "list", "-deps", "-json", mainPackage)
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	cmd.Stderr = os.Stderr

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("go list stdout: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("go list: %w", err)
	}

	decoder := json.NewDecoder(stdout)
	for decoder.More() {
		var pkg goPackage
		if err := decoder.Decode(&pkg); err != nil {
			_ = cmd.Wait()
			return nil, fmt.Errorf("decode go list json: %w", err)
		}

		if pkg.Standard || pkg.Dir == "" {
			continue
		}

		for _, name := range append(append([]string{}, pkg.GoFiles...), pkg.EmbedFiles...) {
			pathSet[filepath.Join(pkg.Dir, name)] = struct{}{}
		}
	}

	if err := cmd.Wait(); err != nil {
		return nil, fmt.Errorf("go list: %w", err)
	}

	paths := make([]string, 0, len(pathSet))
	for path := range pathSet {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	return paths, nil
}

func hashPaths(paths []string) ([]byte, error) {
	hasher := sha256.New()
	tarWriter := tar.NewWriter(hasher)

	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			return nil, fmt.Errorf("stat %s: %w", path, err)
		}
		if !info.Mode().IsRegular() {
			continue
		}

		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return nil, fmt.Errorf("tar header %s: %w", path, err)
		}
		header.Name = path

		if err := tarWriter.WriteHeader(header); err != nil {
			return nil, fmt.Errorf("tar write header %s: %w", path, err)
		}

		file, err := os.Open(path)
		if err != nil {
			return nil, fmt.Errorf("open %s: %w", path, err)
		}

		if _, err := io.Copy(tarWriter, file); err != nil {
			_ = file.Close()
			return nil, fmt.Errorf("tar write body %s: %w", path, err)
		}
		_ = file.Close()
	}

	if err := tarWriter.Close(); err != nil {
		return nil, fmt.Errorf("tar close: %w", err)
	}

	return hasher.Sum(nil), nil
}
