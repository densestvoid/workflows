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

func TestCollectFilesResolvesEmbedFiles(t *testing.T) {
	tests := []struct {
		name    string
		mainGo  string
		files   map[string]string
		want    []string
		missing []string
	}{
		{
			name: "glob",
			mainGo: `package main

import "embed"

//go:embed assets/*
var assets string

func main() {}
`,
			files: map[string]string{
				"helper.go":        "package main\n\nfunc helper() int { return 1 }\n",
				"assets/data.txt":  "payload\n",
			},
			want: []string{"go.mod", "main.go", "helper.go", "data.txt"},
		},
		{
			name: "directory",
			mainGo: `package main

import "embed"

//go:embed templates
var templates embed.FS

func main() {}
`,
			files: map[string]string{
				"templates/nested/page.html": "<html></html>\n",
			},
			want: []string{"page.html"},
		},
		{
			name: "all prefix includes hidden",
			mainGo: `package main

import "embed"

//go:embed all:t
var t embed.FS

func main() {}
`,
			files: map[string]string{
				"t/visible.txt": "ok\n",
				"t/.hidden":     "secret\n",
			},
			want: []string{"visible.txt", ".hidden"},
		},
		{
			name: "directory without all skips hidden",
			mainGo: `package main

import "embed"

//go:embed t
var t embed.FS

func main() {}
`,
			files: map[string]string{
				"t/visible.txt": "ok\n",
				"t/.hidden":     "secret\n",
			},
			want:    []string{"visible.txt"},
			missing: []string{".hidden"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()

			writeFile(t, filepath.Join(dir, "go.mod"), "module example.com/app\n\ngo 1.22\n")
			writeFile(t, filepath.Join(dir, "main.go"), tc.mainGo)
			for path, contents := range tc.files {
				writeFile(t, filepath.Join(dir, path), contents)
			}

			chdir(t, dir)

			got, err := collectFiles(".")
			if err != nil {
				t.Fatal(err)
			}

			assertBasenamesFound(t, got, tc.want...)
			assertBasenamesMissing(t, got, tc.missing...)
		})
	}
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

func assertBasenamesMissing(t *testing.T, paths []string, missing ...string) {
	t.Helper()

	for _, path := range paths {
		base := filepath.Base(path)
		for _, name := range missing {
			if base == name {
				t.Fatalf("expected %q to be absent, got %v", name, paths)
			}
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
