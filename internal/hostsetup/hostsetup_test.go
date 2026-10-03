package hostsetup

import (
	"strings"
	"testing"
)

func TestShQuote(t *testing.T) {
	if got := shQuote("it's"); got != `'it'\''s'` {
		t.Fatal(got)
	}
}

func TestPlatform(t *testing.T) {
	if platform("Linux", "x86_64") != "linux/amd64" || platform("Darwin", "arm64") != "darwin/arm64" || platform("SunOS", "i86pc") != "" {
		t.Fatal("platform mapping")
	}
}

func TestExplain(t *testing.T) {
	if !strings.Contains(Explain("me@x: Permission denied (publickey).", "x"), "key login") {
		t.Fatal("permission hint")
	}
	if !strings.Contains(Explain("sh: 1: exec: gecko: not found", "box"), "gecko host install box") {
		t.Fatal("install hint")
	}
}
