package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSplitTarget(t *testing.T) {
	cases := []struct{ in, dest, port string }{
		{"devbox", "devbox", ""},
		{"me@devbox", "me@devbox", ""},
		{"pwnable.kr:2222", "pwnable.kr", "2222"},
		{"fd@pwnable.kr:2222", "fd@pwnable.kr", "2222"},
		{"ssh://me@host.example:2200/", "me@host.example", "2200"},
		{"[::1]:2222", "::1", "2222"},
		{"fe80::1", "fe80::1", ""},
		{"host:notaport", "host:notaport", ""},
	}
	for _, c := range cases {
		d, p := SplitTarget(c.in)
		if d != c.dest || p != c.port {
			t.Errorf("SplitTarget(%q) = %q, %q; want %q, %q", c.in, d, p, c.dest, c.port)
		}
	}
	argv := Host{Name: "x", SSH: "fd@pwnable.kr:2222"}.BridgeCommand()
	found := false
	for i, a := range argv {
		if a == "-p" && argv[i+1] == "2222" && argv[i+2] == "fd@pwnable.kr" {
			found = true
		}
	}
	if !found {
		t.Fatalf("argv %q", argv)
	}
}

func TestMigrate(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("GECKO_HOME", "")
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("XDG_STATE_HOME", "")
	old := filepath.Join(home, ".config", "gecko")
	os.MkdirAll(old, 0o700)
	os.WriteFile(filepath.Join(old, "config.json"), []byte(`{"hosts":[{"name":"devbox","ssh":"root@devbox"}]}`), 0o600)
	os.WriteFile(filepath.Join(old, "token"), []byte("abc123abc123abc123\n"), 0o600)

	Migrate()
	if Home() != filepath.Join(home, ".gecko-terminal") {
		t.Fatalf("home %s", Home())
	}
	c, err := Load()
	if err != nil || len(c.Hosts) != 1 || c.Hosts[0].Name != "devbox" {
		t.Fatalf("config not migrated: %+v %v", c, err)
	}
	if tok, _ := Token(); tok != "abc123abc123abc123" {
		t.Fatalf("token not migrated: %q", tok)
	}
	if _, err := os.Stat(filepath.Join(old, "config.json")); err != nil {
		t.Fatal("old config should be left in place")
	}
	// A second run doesn't overwrite newer settings.
	os.WriteFile(Path(), []byte(`{}`), 0o600)
	Migrate()
	if b, _ := os.ReadFile(Path()); string(b) != "{}" {
		t.Fatal("migrated twice")
	}
}
