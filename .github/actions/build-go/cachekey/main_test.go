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
	firstHash, err := hashFiles(paths)
	if err != nil {
		t.Fatal(err)
	}

	secondHash, err := hashFiles(paths)
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

	changedHash, err := hashFiles(append(paths, changed))
	if err != nil {
		t.Fatal(err)
	}

	if string(firstHash) == string(changedHash) {
		t.Fatal("expected hash to change when inputs change")
	}
}

func TestCollectFilesIncludesModuleSourcesAndEmbeds(t *testing.T) {
	dir := t.TempDir()

	writeFile(t, filepath.Join(dir, "go.mod"), "module example.com/app\n\ngo 1.22\n")
	writeFile(t, filepath.Join(dir, "main.go"), "package main\n\nimport _ \"embed\"\n\n//go:embed assets/*\nvar assets string\n\nfunc main() {}\n")
	writeFile(t, filepath.Join(dir, "helper.go"), "package main\n\nfunc helper() int { return 1 }\n")
	writeFile(t, filepath.Join(dir, "assets", "data.txt"), "payload\n")

	chdir(t, dir)

	files, err := collectFiles(".")
	if err != nil {
		t.Fatal(err)
	}

	assertBasenamesFound(t, files, "go.mod", "main.go", "helper.go", "data.txt")
}

func TestCollectFilesDirectoryEmbed(t *testing.T) {
	dir := t.TempDir()

	writeFile(t, filepath.Join(dir, "go.mod"), "module example.com/app\n\ngo 1.22\n")
	writeFile(t, filepath.Join(dir, "main.go"), "package main\n\nimport \"embed\"\n\n//go:embed templates\nvar templates embed.FS\n\nfunc main() {}\n")
	writeFile(t, filepath.Join(dir, "templates", "nested", "page.html"), "<html></html>\n")

	chdir(t, dir)

	files, err := collectFiles(".")
	if err != nil {
		t.Fatal(err)
	}

	assertBasenamesFound(t, files, "page.html")
}

func TestCollectFilesAllPrefixIncludesHidden(t *testing.T) {
	dir := t.TempDir()

	writeFile(t, filepath.Join(dir, "go.mod"), "module example.com/app\n\ngo 1.22\n")
	writeFile(t, filepath.Join(dir, "main.go"), "package main\n\nimport \"embed\"\n\n//go:embed all:t\nvar t embed.FS\n\nfunc main() {}\n")
	writeFile(t, filepath.Join(dir, "t", "visible.txt"), "ok\n")
	writeFile(t, filepath.Join(dir, "t", ".hidden"), "secret\n")

	chdir(t, dir)

	files, err := collectFiles(".")
	if err != nil {
		t.Fatal(err)
	}

	assertBasenamesFound(t, files, "visible.txt", ".hidden")
}

func TestCollectFilesDirectoryEmbedSkipsHiddenWithoutAll(t *testing.T) {
	dir := t.TempDir()

	writeFile(t, filepath.Join(dir, "go.mod"), "module example.com/app\n\ngo 1.22\n")
	writeFile(t, filepath.Join(dir, "main.go"), "package main\n\nimport \"embed\"\n\n//go:embed t\nvar t embed.FS\n\nfunc main() {}\n")
	writeFile(t, filepath.Join(dir, "t", "visible.txt"), "ok\n")
	writeFile(t, filepath.Join(dir, "t", ".hidden"), "secret\n")

	chdir(t, dir)

	files, err := collectFiles(".")
	if err != nil {
		t.Fatal(err)
	}

	assertBasenamesFound(t, files, "visible.txt")
	assertBasenamesMissing(t, files, ".hidden")
}

func TestFilesForEmbedPatternAbsolutizedAllPrefix(t *testing.T) {
	dir := t.TempDir()

	writeFile(t, filepath.Join(dir, "t", "x.txt"), "ok\n")
	writeFile(t, filepath.Join(dir, "t", ".hidden"), "secret\n")

	// go/packages absolutizes all:t as <pkgDir>/all:t
	pattern := filepath.Join(dir, "all:t")
	files, err := filesForEmbedPattern(dir, pattern)
	if err != nil {
		t.Fatal(err)
	}

	assertBasenamesFound(t, files, "x.txt", ".hidden")
}

func TestFilesForEmbedPatternGlob(t *testing.T) {
	dir := t.TempDir()

	writeFile(t, filepath.Join(dir, "assets", "data.txt"), "payload\n")

	pattern := filepath.Join(dir, "assets", "*")
	files, err := filesForEmbedPattern(dir, pattern)
	if err != nil {
		t.Fatal(err)
	}

	assertBasenamesFound(t, files, "data.txt")
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

func TestEmbedPatternRelative(t *testing.T) {
	pkgDir := "/proj/pkg"
	tests := []struct {
		pattern string
		want    string
	}{
		{pattern: "/proj/pkg/assets/*", want: "assets/*"},
		{pattern: "/proj/pkg/all:t", want: "all:t"},
		{pattern: "assets/*", want: "assets/*"},
	}

	for _, tc := range tests {
		got := embedPatternRelative(pkgDir, tc.pattern)
		if got != tc.want {
			t.Errorf("embedPatternRelative(%q, %q) = %q, want %q", pkgDir, tc.pattern, got, tc.want)
		}
	}
}
