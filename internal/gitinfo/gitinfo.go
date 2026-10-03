// Package gitinfo finds the git repository and branch for a directory by
// reading .git directly (no git binary, no subprocesses).
package gitinfo

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Info describes the repository containing a directory.
type Info struct {
	Root   string // work tree root
	Name   string // repository name (basename of the main work tree)
	Branch string // branch, or short commit when detached
}

// Lookup returns the repository containing dir, or ok=false.
func Lookup(dir string) (info Info, ok bool) {
	if dir == "" {
		return Info{}, false
	}
	dir = filepath.Clean(dir)
	for {
		gitDir, found := gitDirAt(dir)
		if found {
			info = Info{Root: dir, Name: filepath.Base(dir), Branch: branch(gitDir)}
			// In a linked worktree (.git/worktrees/x), name it after the main repo.
			if i := strings.LastIndex(gitDir, string(filepath.Separator)+filepath.Join(".git", "worktrees")+string(filepath.Separator)); i >= 0 {
				info.Name = filepath.Base(gitDir[:i])
			}
			return info, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return Info{}, false
		}
		dir = parent
	}
}

// gitDirAt returns the git directory for a work tree rooted at dir.
func gitDirAt(dir string) (string, bool) {
	p := filepath.Join(dir, ".git")
	st, err := os.Stat(p)
	if err != nil {
		return "", false
	}
	if st.IsDir() {
		return p, true
	}
	// Worktrees and submodules: ".git" is a file containing "gitdir: <path>".
	b, err := os.ReadFile(p)
	if err != nil {
		return "", false
	}
	g, ok := strings.CutPrefix(strings.TrimSpace(string(b)), "gitdir:")
	if !ok {
		return "", false
	}
	g = strings.TrimSpace(g)
	if !filepath.IsAbs(g) {
		g = filepath.Join(dir, g)
	}
	return filepath.Clean(g), true
}

func branch(gitDir string) string {
	b, err := os.ReadFile(filepath.Join(gitDir, "HEAD"))
	if err != nil {
		return ""
	}
	head := strings.TrimSpace(string(b))
	if ref, ok := strings.CutPrefix(head, "ref:"); ok {
		return strings.TrimPrefix(strings.TrimSpace(ref), "refs/heads/")
	}
	if len(head) >= 7 {
		return head[:7] // detached
	}
	return head
}

// Status is the working-tree state of a repository, as a prompt shows it.
type Status struct {
	Changed   int // staged, modified, renamed or conflicted files
	Untracked int
	Ahead     int
	Behind    int
}

// ReadStatus runs `git status` in root. It never blocks for long: big repos
// that take more than the timeout report ok=false.
func ReadStatus(root string, timeout time.Duration) (st Status, ok bool) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "-C", root, "--no-optional-locks", "status", "--porcelain=v2", "--branch")
	cmd.Env = append(os.Environ(), "GIT_OPTIONAL_LOCKS=0", "LC_ALL=C")
	out, err := cmd.Output()
	if err != nil {
		return Status{}, false
	}
	return ParseStatus(string(out)), true
}

// ParseStatus parses `git status --porcelain=v2 --branch` output.
func ParseStatus(out string) Status {
	var st Status
	for _, l := range strings.Split(out, "\n") {
		switch {
		case strings.HasPrefix(l, "# branch.ab "):
			for _, f := range strings.Fields(l[len("# branch.ab "):]) {
				n, _ := strconv.Atoi(f[1:])
				if f[0] == '+' {
					st.Ahead = n
				} else {
					st.Behind = n
				}
			}
		case strings.HasPrefix(l, "1 "), strings.HasPrefix(l, "2 "), strings.HasPrefix(l, "u "):
			st.Changed++
		case strings.HasPrefix(l, "? "):
			st.Untracked++
		}
	}
	return st
}
