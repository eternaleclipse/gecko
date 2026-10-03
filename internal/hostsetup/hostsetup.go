// Package hostsetup checks that a machine is reachable over SSH and has
// gecko, and installs gecko there. Output is streamed line by line so a UI
// can show exactly what happened.
package hostsetup

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/gecko-term/gecko/internal/config"
)

// Result is what a successful check found.
type Result struct {
	OS, Arch string // gecko naming: linux/amd64, darwin/arm64, ...
	Gecko    string // `gecko version` output; "" when Missing
	Missing  bool   // reachable, but gecko isn't installed
}

// sshOpts are the options gecko always uses: no prompts (nobody is there to
// answer), trust a new host key on first use but refuse a changed one.
var sshOpts = []string{"-o", "BatchMode=yes", "-o", "ConnectTimeout=10", "-o", "StrictHostKeyChecking=accept-new"}

func sshCmd(ctx context.Context, target string, remote string) *exec.Cmd {
	dest, port := config.SplitTarget(target)
	args := append([]string{"-T"}, sshOpts...)
	if port != "" {
		args = append(args, "-p", port)
	}
	return exec.CommandContext(ctx, "ssh", append(args, dest, remote)...)
}

// shQuote quotes s for a POSIX shell.
func shQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'" }

const remotePath = `PATH="$HOME/.local/bin:$HOME/bin:$HOME/go/bin:/usr/local/bin:/opt/homebrew/bin:$PATH"`

// run executes cmd, sending each output line to log, and returns the last
// line of output (useful as an error message).
func run(cmd *exec.Cmd, log func(string)) (last string, err error) {
	pr, pw := io.Pipe()
	cmd.Stdout, cmd.Stderr = pw, pw
	if err := cmd.Start(); err != nil {
		return "", err
	}
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		sc := bufio.NewScanner(pr)
		for sc.Scan() {
			line := strings.TrimRight(sc.Text(), "\r")
			if strings.TrimSpace(line) == "" {
				continue
			}
			last = line
			log(line)
		}
	}()
	err = cmd.Wait()
	pw.Close()
	wg.Wait()
	return last, err
}

// Probe connects to target and reports the platform and gecko version.
func Probe(ctx context.Context, target, geckoPath string, log func(string)) (Result, error) {
	dest, port := config.SplitTarget(target)
	if dest == "" {
		return Result{}, errors.New("enter an SSH destination, like me@devbox or me@devbox:2222")
	}
	where := dest
	if port != "" {
		where += " (port " + port + ")"
	}
	log("$ ssh " + where)
	gecko := "gecko"
	if geckoPath != "" {
		gecko = shQuote(geckoPath)
	}
	script := `uname -sm; ` + remotePath + `; if command -v ` + gecko + ` >/dev/null 2>&1; then echo "GECKO: $(` + gecko + ` version)"; else echo GECKO_MISSING; fi`
	var res Result
	last, err := run(sshCmd(ctx, target, "sh -c "+shQuote(script)), func(line string) {
		switch {
		case strings.HasPrefix(line, "GECKO: "):
			res.Gecko = strings.TrimPrefix(line, "GECKO: ")
			log("found " + res.Gecko)
		case line == "GECKO_MISSING":
			res.Missing = true
			log("gecko is not installed there")
		default:
			if f := strings.Fields(line); res.OS == "" && len(f) == 2 && platform(f[0], f[1]) != "" {
				res.OS, res.Arch = splitPlatform(platform(f[0], f[1]))
			}
			log(line)
		}
	})
	if err != nil {
		if ctx.Err() != nil {
			return res, errors.New("timed out")
		}
		return res, errors.New(Explain(last, ""))
	}
	if res.OS == "" {
		return res, errors.New("connected, but couldn't tell what system this is")
	}
	return res, nil
}

func platform(sys, machine string) string {
	goos := map[string]string{"Linux": "linux", "Darwin": "darwin", "FreeBSD": "freebsd"}[sys]
	goarch := map[string]string{"x86_64": "amd64", "amd64": "amd64", "aarch64": "arm64", "arm64": "arm64"}[machine]
	if goos == "" || goarch == "" {
		return ""
	}
	return goos + "/" + goarch
}

func splitPlatform(p string) (string, string) {
	a, b, _ := strings.Cut(p, "/")
	return a, b
}

// Binary finds a gecko binary for goos/goarch: this one if it matches, else
// one built by `make dist` next to it or in ./dist.
func Binary(goos, goarch string) (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	if goos == runtime.GOOS && goarch == runtime.GOARCH {
		return exe, nil
	}
	for _, dir := range []string{filepath.Dir(exe), "dist"} {
		p := filepath.Join(dir, "gecko-"+goos+"-"+goarch)
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	return "", fmt.Errorf("no gecko build for %s/%s here; run `make dist` in the gecko repo", goos, goarch)
}

// Install copies a matching gecko to ~/.local/bin/gecko on target.
func Install(ctx context.Context, target string, res Result, log func(string)) error {
	bin, err := Binary(res.OS, res.Arch)
	if err != nil {
		return err
	}
	dest, port := config.SplitTarget(target)
	log("$ ssh " + dest + " mkdir -p ~/.local/bin")
	if last, err := run(sshCmd(ctx, target, "mkdir -p ~/.local/bin"), log); err != nil {
		return errors.New(Explain(last, ""))
	}
	user, host := "", dest
	if i := strings.LastIndexByte(dest, '@'); i >= 0 {
		user, host = dest[:i+1], dest[i+1:]
	}
	if strings.Contains(host, ":") {
		host = "[" + host + "]"
	}
	args := append([]string{"-q"}, sshOpts...)
	if port != "" {
		args = append(args, "-P", port)
	}
	args = append(args, bin, user+host+":.local/bin/gecko.new")
	log("$ scp " + filepath.Base(bin) + " " + dest + ":~/.local/bin/gecko")
	if last, err := run(exec.CommandContext(ctx, "scp", args...), log); err != nil {
		return errors.New(Explain(last, ""))
	}
	if last, err := run(sshCmd(ctx, target, "chmod +x ~/.local/bin/gecko.new && mv ~/.local/bin/gecko.new ~/.local/bin/gecko"), log); err != nil {
		return errors.New(Explain(last, ""))
	}
	log("installed")
	return nil
}

// Explain turns common ssh / shell failures into what to do about them.
func Explain(msg, host string) string {
	install := "gecko host install"
	if host != "" {
		install += " " + host
	}
	switch {
	case msg == "":
		return "the connection failed without saying why"
	case strings.Contains(msg, "Permission denied"):
		return msg + ". Gecko connects without a password prompt; set up key login (ssh-copy-id) for this machine"
	case strings.Contains(msg, "gecko: not found"), strings.Contains(msg, "gecko: command not found"), strings.Contains(msg, "exec: gecko"):
		return "Gecko isn't installed there. Run: " + install
	case strings.Contains(msg, "Host key verification failed"), strings.Contains(msg, "REMOTE HOST IDENTIFICATION HAS CHANGED"):
		return msg + ". The host key changed; check it, then fix ~/.ssh/known_hosts"
	case strings.Contains(msg, "Could not resolve hostname"):
		return msg + ". Check the name (use host:port or user@host:port)"
	case strings.Contains(msg, "Connection refused"):
		return msg + ". Nothing is listening for SSH there (wrong port?)"
	case strings.Contains(msg, "timed out"):
		return msg + ". The machine didn't answer; is it on and reachable from here?"
	}
	return msg
}
