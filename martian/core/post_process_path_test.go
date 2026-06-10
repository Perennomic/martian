package core

import (
	"path/filepath"
	"testing"
)

func TestPathIsInsideDir(t *testing.T) {
	root := t.TempDir()
	inside := filepath.Join(root, "files", "out.txt")
	if !pathIsInsideDir(inside, root) {
		t.Fatalf("expected %s to be inside %s", inside, root)
	}
	if !pathIsInsideDir(root, root) {
		t.Fatalf("expected %s to be inside itself", root)
	}

	sibling := root + "-sibling"
	if pathIsInsideDir(filepath.Join(sibling, "out.txt"), root) {
		t.Fatalf("expected sibling path %s not to be inside %s", sibling, root)
	}
}
