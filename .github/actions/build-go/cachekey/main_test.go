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

	paths, err := sourcePaths(".")
	if err != nil {
		t.Fatal(err)
	}

	want := map[string]bool{
		"go.mod":    false,
		"main.go":   false,
		"helper.go": false,
	}
	for _, path := range paths {
		base := filepath.Base(path)
		if _, ok := want[base]; ok {
			want[base] = true
		}
		if path == "go.mod" {
			want["go.mod"] = true
		}
	}

	for path, found := range want {
		if !found {
			t.Fatalf("expected %q in source paths, got %v", path, paths)
		}
	}
}

func writeFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}
