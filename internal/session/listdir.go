package session

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/gecko-term/gecko/internal/proto"
)

// ListDir lists the folders inside dir on this machine ("~" is expanded).
// It returns the cleaned absolute path, the folders, and the home folder.
func ListDir(dir string) (string, []proto.DirEntry, string, error) {
	home, _ := os.UserHomeDir()
	dir = expandHome(strings.TrimSpace(dir))
	if dir == "" {
		dir = home
	}
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(home, dir)
	}
	dir = filepath.Clean(dir)
	ents, err := os.ReadDir(dir)
	if err != nil {
		return dir, nil, home, err
	}
	out := []proto.DirEntry{}
	for _, e := range ents {
		isDir := e.IsDir()
		if !isDir && e.Type()&os.ModeSymlink != 0 {
			if st, err := os.Stat(filepath.Join(dir, e.Name())); err == nil && st.IsDir() {
				isDir = true
			}
		}
		if !isDir {
			continue
		}
		d := proto.DirEntry{Name: e.Name(), Hidden: strings.HasPrefix(e.Name(), ".")}
		if _, err := os.Stat(filepath.Join(dir, e.Name(), ".git")); err == nil {
			d.Git = true
		}
		out = append(out, d)
		if len(out) >= 2000 {
			break
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Hidden != out[j].Hidden {
			return !out[i].Hidden
		}
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	return dir, out, home, nil
}
