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

	paths := []string{second, first}
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
