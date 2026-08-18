package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHashPathsStable(t *testing.T) {
	dir := t.TempDir()

	first := filepath.Join(dir, "first.txt")
	second := filepath.Join(dir, "second.txt")
	if err := os.WriteFile(first, []byte("alpha"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(second, []byte("beta"), 0o644); err != nil {
		t.Fatal(err)
	}

	paths := []string{first, second}
	firstHash, err := hashPaths(paths)
	if err != nil {
		t.Fatal(err)
	}

	secondHash, err := hashPaths(paths)
	if err != nil {
		t.Fatal(err)
	}

	if string(firstHash) != string(secondHash) {
		t.Fatalf("expected stable hash, got %x and %x", firstHash, secondHash)
	}

	changed := filepath.Join(dir, "changed.txt")
	if err := os.WriteFile(changed, []byte("gamma"), 0o644); err != nil {
		t.Fatal(err)
	}

	changedHash, err := hashPaths(append(paths, changed))
	if err != nil {
		t.Fatal(err)
	}

	if string(firstHash) == string(changedHash) {
		t.Fatal("expected hash to change when inputs change")
	}
}

func TestSourcePathsIncludesModuleAndSources(t *testing.T) {
	dir := t.TempDir()

	writeFile(t, filepath.Join(dir, "go.mod"), "module example.com/app\n\ngo 1.22\n")
	writeFile(t, filepath.Join(dir, "main.go"), "package main\n\nfunc main() {}\n")
	writeFile(t, filepath.Join(dir, "helper.go"), "package main\n\nfunc helper() int { return 1 }\n")

	chdir(t, dir)

	paths, err := sourcePaths(".")
	if err != nil {
		t.Fatal(err)
	}

	assertBasenamesFound(t, paths, "go.mod", "main.go", "helper.go")
}

func TestSourcePathsIncludesEmbedPatterns(t *testing.T) {
	dir := t.TempDir()

	writeFile(t, filepath.Join(dir, "go.mod"), "module example.com/app\n\ngo 1.22\n")
	writeFile(t, filepath.Join(dir, "main.go"), "package main\n\nimport _ \"embed\"\n\n//go:embed assets/*\nvar assets string\n\nfunc main() {}\n")
	writeFile(t, filepath.Join(dir, "assets", "data.txt"), "payload\n")

	chdir(t, dir)

	paths, err := sourcePaths(".")
	if err != nil {
		t.Fatal(err)
	}

	assertBasenamesFound(t, paths, "go.mod", "main.go", "data.txt")
}

func TestExpandEmbedPatternRecursive(t *testing.T) {
	dir := t.TempDir()
	nested := filepath.Join(dir, "static", "nested")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(nested, "page.html"), "<html></html>\n")

	matches, err := expandEmbedPattern(filepath.Join(dir, "static", "..."))
	if err != nil {
		t.Fatal(err)
	}

	assertBasenamesFound(t, matches, "page.html")
}

func assertBasenamesFound(t *testing.T, paths []string, want ...string) {
	t.Helper()

	found := make(map[string]bool, len(want))
	for _, name := range want {
		found[name] = false
	}

	for _, path := range paths {
		base := filepath.Base(path)
		if _, ok := found[base]; ok {
			found[base] = true
		}
	}

	for name, ok := range found {
		if !ok {
			t.Fatalf("expected %q in paths, got %v", name, paths)
		}
	}
}

func chdir(t *testing.T, dir string) {
	t.Helper()

	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(originalWD)
	})
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
}

func writeFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}
