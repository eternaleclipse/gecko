// Package config loads Gecko's configuration and knows where its files live.
package config

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/gecko-term/gecko/internal/proto"
)

// Host is a remote machine reached through SSH (or any command that runs
// `gecko bridge` on the other side).
type Host struct {
	Name    string   `json:"name"`
	SSH     string   `json:"ssh,omitempty"`     // ssh destination, e.g. me@devbox
	Gecko   string   `json:"gecko,omitempty"`   // path to gecko on the remote
	Command []string `json:"command,omitempty"` // custom transport, overrides SSH
}

// Config is ~/.config/gecko/config.json.
type Config struct {
	Name            string            `json:"name,omitempty"`   // this machine's name
	Listen          string            `json:"listen,omitempty"` // web UI address
	Shell           string            `json:"shell,omitempty"`
	ScrollbackBytes int               `json:"scrollbackBytes,omitempty"`
	Hosts           []Host            `json:"hosts,omitempty"`
	Workspaces      []proto.Workspace `json:"workspaces,omitempty"`
	Agents          map[string]string `json:"agents,omitempty"` // extra executable -> agent name
	Notify          struct {
		Webhook string `json:"webhook,omitempty"` // POSTed when an agent needs input (e.g. ntfy.sh)
	} `json:"notify,omitempty"`

	mu sync.Mutex
}

// Home is where all of Gecko's files live: ~/.gecko-terminal (also on
// Windows, under the user profile). GECKO_HOME overrides it.
//
//	config.json    machines, workspaces, listen address, ...
//	settings.json  interface settings and key bindings (synced to every window)
//	token          web access token
//	state/         socket, logs, window size, shell integration scripts
func Home() string {
	if h := os.Getenv("GECKO_HOME"); h != "" {
		return h
	}
	h, _ := os.UserHomeDir()
	return filepath.Join(h, ".gecko-terminal")
}

// StateDir holds runtime files: the socket, logs, shell integration scripts.
func StateDir() string { return filepath.Join(Home(), "state") }

// SettingsPath is the interface settings file.
func SettingsPath() string { return filepath.Join(Home(), "settings.json") }

// legacyDirs are where Gecko kept its files before ~/.gecko-terminal.
func legacyDirs() (conf, state string) {
	if runtime.GOOS == "windows" {
		c, _ := os.UserConfigDir()
		d, _ := os.UserCacheDir()
		return filepath.Join(c, "gecko"), filepath.Join(d, "gecko")
	}
	h, _ := os.UserHomeDir()
	conf, state = filepath.Join(h, ".config", "gecko"), filepath.Join(h, ".local", "state", "gecko")
	if x := os.Getenv("XDG_CONFIG_HOME"); x != "" {
		conf = filepath.Join(x, "gecko")
	}
	if x := os.Getenv("XDG_STATE_HOME"); x != "" {
		state = filepath.Join(x, "gecko")
	}
	return conf, state
}

// LegacySocket is the socket of a daemon started before the move to
// ~/.gecko-terminal, so clients can still reach it until it restarts.
func LegacySocket() string {
	if os.Getenv("GECKO_HOME") != "" {
		return ""
	}
	_, state := legacyDirs()
	return filepath.Join(state, "gecko.sock")
}

// Migrate copies config, token and window size from the old locations into
// ~/.gecko-terminal the first time it runs. The old files are left alone.
func Migrate() {
	if os.Getenv("GECKO_HOME") != "" {
		return
	}
	if _, err := os.Stat(Home()); err == nil {
		return
	}
	conf, state := legacyDirs()
	copies := map[string]string{
		filepath.Join(conf, "config.json"):  Path(),
		filepath.Join(conf, "token"):        filepath.Join(Home(), "token"),
		filepath.Join(state, "window.json"): filepath.Join(StateDir(), "window.json"),
	}
	for from, to := range copies {
		b, err := os.ReadFile(from)
		if err != nil {
			continue
		}
		_ = os.MkdirAll(filepath.Dir(to), 0o700)
		_ = os.WriteFile(to, b, 0o600)
	}
	_ = os.MkdirAll(StateDir(), 0o700)
}

// Socket is the daemon's control socket. Unix socket paths are limited to
// ~104 bytes, so deep state dirs fall back to a short per-dir path in /tmp.
func Socket() string {
	p := filepath.Join(StateDir(), "gecko.sock")
	if len(p) <= 100 {
		return p
	}
	sum := sha256.Sum256([]byte(StateDir()))
	return filepath.Join(os.TempDir(), fmt.Sprintf("gecko-%d-%x", os.Getuid(), sum[:5]), "gecko.sock")
}

// Path is the config file.
func Path() string { return filepath.Join(Home(), "config.json") }

// Load reads the config, returning defaults if it doesn't exist.
func Load() (*Config, error) {
	Migrate()
	c := &Config{}
	b, err := os.ReadFile(Path())
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	if len(b) > 0 {
		if err := json.Unmarshal(b, c); err != nil {
			return nil, errors.New(Path() + ": " + err.Error())
		}
	}
	if c.Name == "" {
		c.Name = DefaultName()
	}
	if c.Listen == "" {
		c.Listen = "127.0.0.1:7681"
	}
	return c, nil
}

// DefaultName is the short host name.
func DefaultName() string {
	h, _ := os.Hostname()
	h, _, _ = strings.Cut(h, ".")
	if h == "" {
		h = "local"
	}
	return strings.ToLower(h)
}

// Save writes the config back.
func (c *Config) Save() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := os.MkdirAll(Home(), 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	tmp := Path() + ".tmp"
	if err := os.WriteFile(tmp, append(b, '\n'), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, Path())
}

// Token returns the web UI access token, creating it on first use.
func Token() (string, error) {
	p := filepath.Join(Home(), "token")
	if b, err := os.ReadFile(p); err == nil && len(strings.TrimSpace(string(b))) >= 16 {
		return strings.TrimSpace(string(b)), nil
	}
	if err := os.MkdirAll(Home(), 0o700); err != nil {
		return "", err
	}
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	t := hex.EncodeToString(buf)
	return t, os.WriteFile(p, []byte(t+"\n"), 0o600)
}

// BridgeCommand returns the argv that connects to host's daemon.
func (h Host) BridgeCommand() []string {
	if len(h.Command) > 0 {
		return h.Command
	}
	gecko := h.Gecko
	if gecko == "" {
		gecko = "gecko"
	}
	remote := `sh -c 'PATH="$HOME/.local/bin:$HOME/bin:$HOME/go/bin:/usr/local/bin:/opt/homebrew/bin:$PATH"; exec ` + gecko + ` bridge'`
	dest, port := SplitTarget(h.SSH)
	argv := []string{"ssh", "-T", "-o", "BatchMode=yes", "-o", "ConnectTimeout=10",
		// Nobody is there to answer "trust this host?", so trust a new host
		// on first use, but still refuse one whose key changed.
		"-o", "StrictHostKeyChecking=accept-new",
		"-o", "ServerAliveInterval=10", "-o", "ServerAliveCountMax=3"}
	if port != "" {
		argv = append(argv, "-p", port)
	}
	return append(argv, dest, remote)
}

// SplitTarget turns "user@host:port", "ssh://user@host:port" or
// "[::1]:2222" into an ssh destination and a port ("" for the default).
func SplitTarget(t string) (dest, port string) {
	t = strings.TrimSuffix(strings.TrimPrefix(strings.TrimSpace(t), "ssh://"), "/")
	user := ""
	if i := strings.LastIndexByte(t, '@'); i >= 0 {
		user, t = t[:i+1], t[i+1:]
	}
	host := t
	if strings.HasPrefix(t, "[") { // [ipv6] or [ipv6]:port
		if end := strings.IndexByte(t, ']'); end > 0 {
			host = t[1:end]
			if rest := t[end+1:]; strings.HasPrefix(rest, ":") && isPort(rest[1:]) {
				port = rest[1:]
			}
		}
	} else if i := strings.LastIndexByte(t, ':'); i > 0 && strings.Count(t, ":") == 1 && isPort(t[i+1:]) {
		host, port = t[:i], t[i+1:]
	}
	return user + host, port
}

func isPort(s string) bool {
	if s == "" || len(s) > 5 {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}
