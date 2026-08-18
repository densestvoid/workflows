package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// filesForEmbedPattern expands one //go:embed pattern to absolute file paths
// using the same stdlib primitives as cmd/go's resolveEmbed (filepath.Glob,
// filepath.WalkDir). pkgDir is the package directory; pattern may be relative
// or absolute as returned by go/packages.
func filesForEmbedPattern(pkgDir, pattern string) ([]string, error) {
	rel := embedPatternRelative(pkgDir, pattern)

	all := strings.HasPrefix(rel, "all:")
	glob := rel
	if all {
		glob = rel[len("all:"):]
	}

	matches, err := filepath.Glob(filepath.Join(pkgDir, filepath.FromSlash(glob)))
	if err != nil {
		return nil, fmt.Errorf("glob %q: %w", pattern, err)
	}

	fileSet := make(map[string]struct{})
	for _, match := range matches {
		info, err := os.Lstat(match)
		if err != nil {
			return nil, fmt.Errorf("stat %s: %w", match, err)
		}

		switch {
		case info.Mode().IsRegular():
			fileSet[match] = struct{}{}

		case info.IsDir():
			err := filepath.WalkDir(match, func(path string, entry fs.DirEntry, walkErr error) error {
				if walkErr != nil {
					return walkErr
				}

				name := entry.Name()
				if path != match && len(name) > 0 && (name[0] == '.' || name[0] == '_') && !all {
					if entry.IsDir() {
						return fs.SkipDir
					}
					return nil
				}

				if entry.IsDir() {
					return nil
				}

				info, err := entry.Info()
				if err != nil {
					return err
				}
				if !info.Mode().IsRegular() {
					return nil
				}

				fileSet[path] = struct{}{}
				return nil
			})
			if err != nil {
				return nil, fmt.Errorf("walk %s: %w", match, err)
			}
		}
	}

	files := make([]string, 0, len(fileSet))
	for path := range fileSet {
		files = append(files, path)
	}
	return files, nil
}

func embedPatternRelative(pkgDir, pattern string) string {
	if !filepath.IsAbs(pattern) {
		return filepath.ToSlash(pattern)
	}

	rel, err := filepath.Rel(pkgDir, pattern)
	if err == nil {
		return filepath.ToSlash(rel)
	}

	prefix := pkgDir + string(filepath.Separator)
	if strings.HasPrefix(pattern, prefix) {
		return filepath.ToSlash(pattern[len(prefix):])
	}

	return filepath.ToSlash(pattern)
}
