// Command cachekey fingerprints a Go main package for build-go binary caching.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"hash"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/tools/go/packages"
)

func main() {
	var mainPackage string
	flag.StringVar(&mainPackage, "main-package", "", "main package path (e.g. ./cmd/server)")
	flag.Parse()

	if mainPackage == "" {
		fmt.Fprintln(os.Stderr, "cachekey: --main-package is required")
		os.Exit(2)
	}

	key, err := fingerprint(mainPackage)
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
	seenModules := make(map[string]struct{})

	cfg := &packages.Config{
		Mode: packages.NeedName |
			packages.NeedFiles |
			packages.NeedEmbedFiles |
			packages.NeedEmbedPatterns |
			packages.NeedDeps |
			packages.NeedModule,
		Env: append(os.Environ(), "CGO_ENABLED=0"),
	}

	pkgs, err := packages.Load(cfg, mainPackage)
	if err != nil {
		return nil, fmt.Errorf("load packages: %w", err)
	}

	for _, pkg := range pkgs {
		for _, loadErr := range pkg.Errors {
			return nil, fmt.Errorf("%s: %s", pkg.PkgPath, loadErr.Msg)
		}

		// Module is nil for standard library packages (see packages.Package docs).
		if pkg.Module == nil {
			continue
		}

		if pkg.Module.GoMod != "" {
			if _, seen := seenModules[pkg.Module.GoMod]; !seen {
				seenModules[pkg.Module.GoMod] = struct{}{}
				pathSet[pkg.Module.GoMod] = struct{}{}

				goSum := filepath.Join(filepath.Dir(pkg.Module.GoMod), "go.sum")
				if _, err := os.Stat(goSum); err == nil {
					pathSet[goSum] = struct{}{}
				}
			}
		}

		for _, path := range pkg.GoFiles {
			pathSet[path] = struct{}{}
		}
		for _, path := range pkg.EmbedFiles {
			pathSet[path] = struct{}{}
		}
		if err := addEmbedPatternPaths(pathSet, pkg.EmbedPatterns); err != nil {
			return nil, err
		}
	}

	paths := make([]string, 0, len(pathSet))
	for path := range pathSet {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	return paths, nil
}

func addEmbedPatternPaths(pathSet map[string]struct{}, patterns []string) error {
	for _, pattern := range patterns {
		matches, err := expandEmbedPattern(pattern)
		if err != nil {
			return fmt.Errorf("expand embed pattern %q: %w", pattern, err)
		}

		for _, match := range matches {
			pathSet[match] = struct{}{}
		}
	}

	return nil
}

func expandEmbedPattern(pattern string) ([]string, error) {
	if pattern == "" {
		return nil, nil
	}

	if strings.HasSuffix(pattern, "/...") {
		root := strings.TrimSuffix(pattern, "/...")
		var matches []string
		err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() {
				return nil
			}
			matches = append(matches, path)
			return nil
		})
		if err != nil {
			if os.IsNotExist(err) {
				return []string{pattern}, nil
			}
			return nil, err
		}
		if len(matches) == 0 {
			return []string{pattern}, nil
		}
		return matches, nil
	}

	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}
	if len(matches) > 0 {
		return matches, nil
	}

	if _, err := os.Stat(pattern); err == nil {
		return []string{pattern}, nil
	}

	// Fingerprint the pattern itself when it has no matches yet.
	return []string{pattern}, nil
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

	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			// Embed pattern with no matching files yet.
			_, writeErr := io.WriteString(hasher, "pattern")
			return writeErr
		}
		return fmt.Errorf("stat %s: %w", path, err)
	}
	if info.IsDir() {
		return fmt.Errorf("hash %s: directories are not fingerprinted directly", path)
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
