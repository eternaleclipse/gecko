package gitinfo

import (
	"os"
	"path/filepath"
	"testing"
)

func write(t *testing.T, p, s string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestParseStatus(t *testing.T) {
	out := `# branch.oid 3f2a91c
# branch.head main
# branch.upstream origin/main
# branch.ab +2 -1
1 .M N... 100644 100644 100644 abc abc web/src/main.js
1 M. N... 100644 100644 100644 abc abc README.md
2 R. N... 100644 100644 100644 abc abc R100 new.go	old.go
? scratch.txt
`
	st := ParseStatus(out)
	if st != (Status{Changed: 3, Untracked: 1, Ahead: 2, Behind: 1}) {
		t.Fatalf("got %+v", st)
	}
}

func TestLookup(t *testing.T) {
	tmp := t.TempDir()
	repo := filepath.Join(tmp, "myrepo")
	write(t, filepath.Join(repo, ".git", "HEAD"), "ref: refs/heads/feature/login\n")
	deep := filepath.Join(repo, "src", "pkg", "auth")
	os.MkdirAll(deep, 0o755)

	info, ok := Lookup(deep)
	if !ok || info.Name != "myrepo" || info.Branch != "feature/login" || info.Root != repo {
		t.Fatalf("got %+v %v", info, ok)
	}

	// Detached HEAD shows a short commit.
	write(t, filepath.Join(repo, ".git", "HEAD"), "3f2a91c0d4e5b6a7\n")
	if info, _ := Lookup(repo); info.Branch != "3f2a91c" {
		t.Fatalf("detached: %+v", info)
	}

	// Linked worktree: named after the main repository.
	wt := filepath.Join(tmp, "myrepo-hotfix")
	write(t, filepath.Join(repo, ".git", "worktrees", "myrepo-hotfix", "HEAD"), "ref: refs/heads/hotfix\n")
	write(t, filepath.Join(wt, ".git"), "gitdir: "+filepath.Join(repo, ".git", "worktrees", "myrepo-hotfix")+"\n")
	if info, ok := Lookup(filepath.Join(wt)); !ok || info.Name != "myrepo" || info.Branch != "hotfix" || info.Root != wt {
		t.Fatalf("worktree: %+v %v", info, ok)
	}

	if _, ok := Lookup(filepath.Join(tmp, "nowhere")); ok {
		t.Fatal("expected no repo")
	}
}
