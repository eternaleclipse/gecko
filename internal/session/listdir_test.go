package session

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestListDir(t *testing.T) {
	root := t.TempDir()
	for _, d := range []string{"b", "A", ".hidden", "repo/.git"} {
		os.MkdirAll(filepath.Join(root, d), 0o755)
	}
	os.WriteFile(filepath.Join(root, "file.txt"), nil, 0o644)
	os.Symlink(filepath.Join(root, "b"), filepath.Join(root, "link"))
	dir, ents, _, err := ListDir(root + "/./")
	if err != nil || dir != root {
		t.Fatalf("dir %q err %v", dir, err)
	}
	var names []string
	for _, e := range ents {
		names = append(names, e.Name)
	}
	if got := fmt.Sprint(names); got != "[A b link repo .hidden]" {
		t.Fatalf("got %s", got)
	}
	if !ents[3].Git || !ents[4].Hidden {
		t.Fatalf("flags: %+v", ents)
	}
}
