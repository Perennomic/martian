package util

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseMroPathUsesPlatformPathListSeparator(t *testing.T) {
	first := filepath.Join("first dir")
	second := filepath.Join("second")
	mroPath := first + string(os.PathListSeparator) + second

	paths := ParseMroPath(mroPath)
	if len(paths) != 2 {
		t.Fatalf("expected two paths, got %#v", paths)
	}
	if paths[0] != first || paths[1] != second {
		t.Fatalf("expected %#v and %#v, got %#v", first, second, paths)
	}
}

func TestParseMroPathEmpty(t *testing.T) {
	if paths := ParseMroPath(""); len(paths) != 0 {
		t.Fatalf("expected empty MROPATH to return no paths, got %#v", paths)
	}
}

func TestFormatMroPathUsesPlatformPathListSeparator(t *testing.T) {
	paths := []string{filepath.Join("first dir"), filepath.Join("second")}
	formatted := FormatMroPath(paths)

	if formatted != strings.Join(paths, string(os.PathListSeparator)) {
		t.Fatalf("unexpected formatted MROPATH: %q", formatted)
	}
}

func TestSearchPathsUsesFilepathJoin(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "stage.mro"), []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	if found, ok := SearchPaths("stage.mro", []string{dir}); !ok {
		t.Fatal("expected file to be found")
	} else if found != filepath.Join(dir, "stage.mro") {
		t.Fatalf("expected filepath-joined path, got %q", found)
	}
}

func TestFindUniquePathUsesFilepathJoin(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "pipeline.mro"), []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	if found, err := FindUniquePath("pipeline.mro", []string{dir}); err != nil {
		t.Fatal(err)
	} else if found != filepath.Join(dir, "pipeline.mro") {
		t.Fatalf("expected filepath-joined path, got %q", found)
	}
}
