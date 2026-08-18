// Command cachekey fingerprints a Go main package for build-go binary caching.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"go/build"
	"hash"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/tools/go/packages"
)

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

	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedEmbedFiles | packages.NeedDeps,
		Env:  append(os.Environ(), "CGO_ENABLED=0"),
	}

	pkgs, err := packages.Load(cfg, mainPackage)
	if err != nil {
		return nil, fmt.Errorf("load packages: %w", err)
	}

	for _, pkg := range pkgs {
		for _, loadErr := range pkg.Errors {
			return nil, fmt.Errorf("%s: %s", pkg.PkgPath, loadErr.Msg)
		}

		if isStandardLibrary(pkg) {
			continue
		}

		for _, path := range append(pkg.GoFiles, pkg.EmbedFiles...) {
			pathSet[path] = struct{}{}
		}
	}

	paths := make([]string, 0, len(pathSet))
	for path := range pathSet {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	return paths, nil
}

func isStandardLibrary(pkg *packages.Package) bool {
	if pkg.Module != nil {
		return false
	}

	if len(pkg.GoFiles) == 0 {
		return pkg.PkgPath == "unsafe" || pkg.PkgPath == "C"
	}

	goroot := filepath.Clean(build.Default.GOROOT) + string(filepath.Separator)
	for _, file := range pkg.GoFiles {
		if !strings.HasPrefix(filepath.Clean(file), goroot) {
			return false
		}
	}

	return true
}

func hashPaths(paths []string) ([]byte, error) {
	hasher := sha256.New()

	for _, path := range paths {
		if err := hashFile(hasher, path); err != nil {
			return nil, err
		}
	}

	return hasher.Sum(nil), nil
}

func hashFile(hasher hash.Hash, path string) error {
	if _, err := io.WriteString(hasher, path); err != nil {
		return fmt.Errorf("hash path %s: %w", path, err)
	}
	if _, err := hasher.Write([]byte{0}); err != nil {
		return fmt.Errorf("hash path %s: %w", path, err)
	}

	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open %s: %w", path, err)
	}
	defer file.Close()

	if _, err := io.Copy(hasher, file); err != nil {
		return fmt.Errorf("hash contents %s: %w", path, err)
	}

	return nil
}
