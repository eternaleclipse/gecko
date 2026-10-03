package hub

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/gecko-term/gecko/internal/config"
	"github.com/gecko-term/gecko/internal/proto"
)

// Interface settings (theme, font size, key bindings, ...) live in
// ~/.gecko-terminal/settings.json. Every window gets them on connect and
// whenever they change, from another window or from an edit to the file.

// Settings returns the settings JSON ("{}" when there are none yet).
func (h *Hub) Settings() string {
	b, err := os.ReadFile(config.SettingsPath())
	if err != nil || !json.Valid(b) {
		return "{}"
	}
	return string(bytes.TrimSpace(b))
}

// SetSettings saves a settings object and sends it to every window.
func (h *Hub) SetSettings(text string) error {
	var obj map[string]any
	if err := json.Unmarshal([]byte(text), &obj); err != nil {
		return errors.New("settings must be a JSON object")
	}
	pretty, _ := json.MarshalIndent(obj, "", "  ")
	pretty = append(pretty, '\n')
	p := config.SettingsPath()
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, pretty, 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, p); err != nil {
		return err
	}
	h.mu.Lock()
	if st, err := os.Stat(p); err == nil {
		h.settingsMod = st.ModTime()
	}
	h.mu.Unlock()
	h.publishAll(&proto.Msg{T: proto.TSettings, Text: string(bytes.TrimSpace(pretty))})
	return nil
}

// watchSettings notices hand edits to settings.json and pushes them out.
func (h *Hub) watchSettings() {
	for {
		time.Sleep(1500 * time.Millisecond)
		st, err := os.Stat(config.SettingsPath())
		if err != nil {
			continue
		}
		h.mu.Lock()
		changed := !st.ModTime().Equal(h.settingsMod)
		h.settingsMod = st.ModTime()
		h.mu.Unlock()
		if changed {
			if b, err := os.ReadFile(config.SettingsPath()); err == nil && json.Valid(b) {
				h.publishAll(&proto.Msg{T: proto.TSettings, Text: string(bytes.TrimSpace(b))})
			}
		}
	}
}

// publishAll sends m to every subscriber that talks to a person (not to
// remote hubs, which have their own settings).
func (h *Hub) publishAll(m *proto.Msg) {
	h.mu.Lock()
	var fns []func(*proto.Msg)
	for _, s := range h.subs {
		if s.scope != ScopeLocal {
			fns = append(fns, s.fn)
		}
	}
	h.mu.Unlock()
	for _, fn := range fns {
		fn(m)
	}
}
