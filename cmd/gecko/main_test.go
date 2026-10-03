package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestWindowArgs(t *testing.T) {
	home := t.TempDir()
	t.Setenv("GECKO_HOME", home)
	if got := windowArgs(); !reflect.DeepEqual(got, []string{"--window-size=900,500"}) {
		t.Fatalf("first launch: %v", got)
	}
	os.MkdirAll(filepath.Join(home, "state"), 0o700)
	os.WriteFile(filepath.Join(home, "state", "window.json"), []byte(`{"w":848,"h":537,"sx":1920,"sy":0,"sw":2560,"sh":1400}`), 0o600)
	want := []string{"--window-size=848,537", "--window-position=2776,431"} // centered on the second monitor
	if got := windowArgs(); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}
