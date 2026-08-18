package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHashFilesStable(t *testing.T) {
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
	firstHash, err := hashFingerprint("go1.22.0", paths)
	if err != nil {
		t.Fatal(err)
	}

	secondHash, err := hashFingerprint("go1.22.0", paths)
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

	changedHash, err := hashFingerprint("go1.22.0", append(paths, changed))
	if err != nil {
		t.Fatal(err)
	}

	if string(firstHash) == string(changedHash) {
		t.Fatal("expected hash to change when inputs change")
	}
}

func TestHashFingerprintIncludesGoVersion(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "only.txt")
	if err := os.WriteFile(path, []byte("same"), 0o644); err != nil {
		t.Fatal(err)
	}

	first, err := hashFingerprint("go1.22.0", []string{path})
	if err != nil {
		t.Fatal(err)
	}

	second, err := hashFingerprint("go1.22.1", []string{path})
	if err != nil {
		t.Fatal(err)
	}

	if string(first) == string(second) {
		t.Fatal("expected hash to change when go version changes")
	}
}
