// Command cachekey fingerprints a Go main package for build-go binary caching.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"hash"
	"io"
	"os"
	"path/filepath"
	"sort"

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
	files, patterns, err := collectInputs(mainPackage)
	if err != nil {
		return "", err
	}

	sum, err := hashInputs(files, patterns)
	if err != nil {
		return "", err
	}

	return hex.EncodeToString(sum), nil
}

func collectInputs(mainPackage string) ([]string, []string, error) {
	fileSet := make(map[string]struct{})
	patternSet := make(map[string]struct{})
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
		return nil, nil, fmt.Errorf("load packages: %w", err)
	}

	for _, pkg := range pkgs {
		for _, loadErr := range pkg.Errors {
			return nil, nil, fmt.Errorf("%s: %s", pkg.PkgPath, loadErr.Msg)
		}

		// Module is nil for standard library packages (see packages.Package docs).
		if pkg.Module == nil {
			continue
		}

		if pkg.Module.GoMod != "" {
			if _, seen := seenModules[pkg.Module.GoMod]; !seen {
				seenModules[pkg.Module.GoMod] = struct{}{}
				fileSet[pkg.Module.GoMod] = struct{}{}

				goSum := filepath.Join(filepath.Dir(pkg.Module.GoMod), "go.sum")
				if _, err := os.Stat(goSum); err == nil {
					fileSet[goSum] = struct{}{}
				}
			}
		}

		for _, path := range pkg.GoFiles {
			fileSet[path] = struct{}{}
		}
		for _, path := range pkg.EmbedFiles {
			fileSet[path] = struct{}{}
		}
		for _, pattern := range pkg.EmbedPatterns {
			patternSet[pattern] = struct{}{}
		}
	}

	return sortedKeys(fileSet), sortedKeys(patternSet), nil
}

func sortedKeys(set map[string]struct{}) []string {
	keys := make([]string, 0, len(set))
	for key := range set {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func hashInputs(files, patterns []string) ([]byte, error) {
	hasher := sha256.New()

	for _, path := range files {
		if err := hashFile(hasher, path); err != nil {
			return nil, err
		}
	}
	for _, pattern := range patterns {
		if err := hashLiteral(hasher, pattern); err != nil {
			return nil, err
		}
	}

	return hasher.Sum(nil), nil
}

func hashLiteral(hasher hash.Hash, value string) error {
	if _, err := io.WriteString(hasher, value); err != nil {
		return fmt.Errorf("hash literal %q: %w", value, err)
	}
	_, err := hasher.Write([]byte{0})
	return err
}

func hashFile(hasher hash.Hash, path string) error {
	if err := hashLiteral(hasher, path); err != nil {
		return err
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
