// Command gecko is a modern terminal for shells, tmux, SSH/mosh and coding
// agents across many machines. See `gecko help`.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"golang.org/x/term"

	"github.com/gecko-term/gecko/internal/attach"
	"github.com/gecko-term/gecko/internal/client"
	"github.com/gecko-term/gecko/internal/config"
	"github.com/gecko-term/gecko/internal/daemon"
	"github.com/gecko-term/gecko/internal/hostsetup"
	"github.com/gecko-term/gecko/internal/proto"
	"github.com/gecko-term/gecko/internal/server"
	"github.com/gecko-term/gecko/internal/session"
)

var version = "dev"

const usage = `gecko - a terminal for shells, tmux, ssh/mosh and coding agents, everywhere

Usage:
  gecko                      start the daemon if needed and open the app
  gecko open | url           open the app / print its URL (with access token)
  gecko ls [--json]          list sessions on all hosts
  gecko new [flags] [-- cmd] create a session (-w workspace -n name -H host -C dir -a attach --exec)
  gecko attach <session>     attach this terminal (detach with Ctrl-\)
  gecko send <session> text  type text into a session (adds Enter; --no-enter)
  gecko capture <session>    print a session's text (tmux scrollback aware)
  gecko kill <session>       end a session
  gecko agents [--json]      coding agents and what they are doing
  gecko ws open <name>       open a workspace template from the config
  gecko host ls|add|rm|install   manage remote machines (ssh)
  gecko daemon [--listen a]  run the daemon in the foreground
  gecko upgrade              restart the daemon with this binary, keeping every session
  gecko stop                 stop the daemon (ends all local sessions)
  gecko service install      start the daemon at login (systemd/launchd)
  gecko bridge               (internal) connect stdio to the daemon; used over ssh
  gecko version

Sessions can be named by id (host/abcd1234), id prefix, or name.
Config: ` + "%s" + `
`

func main() {
	session.Version = version
	args := os.Args[1:]
	cmd := ""
	if len(args) > 0 {
		cmd, args = args[0], args[1:]
	}
	var err error
	switch cmd {
	case "":
		err = cmdDefault()
	case "help", "-h", "--help":
		fmt.Printf(usage, config.Path())
	case "version", "--version":
		fmt.Println("gecko", version, runtime.GOOS+"/"+runtime.GOARCH)
	case "daemon":
		err = cmdDaemon(args)
	case "open":
		err = cmdOpen()
	case "url":
		err = cmdURL()
	case "ls", "list":
		err = cmdList(args)
	case "new":
		err = cmdNew(args)
	case "attach", "a":
		err = withSession(args, func(c *client.Client, id string) error { return attach.Run(c, id) })
	case "kill":
		err = withSession(args, func(c *client.Client, id string) error {
			_, err := c.Call(&proto.Msg{T: proto.TKill, ID: id})
			return err
		})
	case "capture":
		err = withSession(args, func(c *client.Client, id string) error {
			r, err := c.Call(&proto.Msg{T: proto.TCapture, ID: id})
			if err == nil {
				fmt.Println(r.Text)
			}
			return err
		})
	case "send":
		err = cmdSend(args)
	case "agents":
		err = cmdAgents(args)
	case "ws", "workspace":
		err = cmdWorkspace(args)
	case "host", "hosts":
		err = cmdHost(args)
	case "bridge":
		err = cmdBridge()
	case "stop":
		err = cmdStop()
	case "upgrade", "restart":
		err = cmdUpgrade(args)
	case "service":
		err = cmdService(args)
	default:
		err = fmt.Errorf("unknown command %q (see gecko help)", cmd)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "gecko:", err)
		os.Exit(1)
	}
}

func headless() bool {
	if os.Getenv("SSH_CONNECTION") != "" {
		return true
	}
	return runtime.GOOS == "linux" && os.Getenv("DISPLAY") == "" && os.Getenv("WAYLAND_DISPLAY") == ""
}

func cmdDefault() error {
	c, err := client.Connect(true)
	if err != nil {
		return err
	}
	c.Close()
	if headless() {
		fmt.Println("gecko daemon is running. Over SSH, use `gecko attach <session>` or open the web UI:")
		if err := cmdURL(); err != nil {
			return err
		}
		fmt.Println()
		return cmdList(nil)
	}
	return cmdOpen()
}

func cmdDaemon(args []string) error {
	fs := flag.NewFlagSet("daemon", flag.ExitOnError)
	listen := fs.String("listen", "", "web UI address (default from config, 127.0.0.1:7681; \"off\" to disable)")
	_ = fs.Parse(args)
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	return daemon.Run(cfg, *listen)
}

func appURL() (string, error) {
	if _, err := client.Connect(true); err != nil {
		return "", err
	}
	tok, err := config.Token()
	if err != nil {
		return "", err
	}
	host, port, err := net.SplitHostPort(daemon.WebAddr())
	if err != nil {
		return "", err
	}
	if ip := net.ParseIP(host); host == "" || ip != nil && ip.IsUnspecified() {
		host = "127.0.0.1"
	}
	return "http://" + net.JoinHostPort(host, port) + "/?token=" + tok, nil
}

func cmdURL() error {
	u, err := appURL()
	if err != nil {
		return err
	}
	fmt.Println(u)
	tok, _ := config.Token()
	for _, s := range server.ShareURLs(daemon.WebAddr(), tok) {
		fmt.Println(s, "  (other devices)")
	}
	return nil
}

// desktopApp finds the desktop app and checks that its sandbox can start;
// otherwise `gecko open` uses a browser app window instead of failing.
func desktopApp() (string, bool) {
	p, err := exec.LookPath("gecko-desktop")
	if err != nil {
		return "", false
	}
	if runtime.GOOS != "linux" || os.Getenv("GECKO_DESKTOP_NO_SANDBOX") == "1" {
		return p, true
	}
	// Ubuntu 23.10+ blocks the user namespaces Chromium's sandbox uses,
	// unless an AppArmor profile allows them or the setuid helper is set up.
	if b, err := os.ReadFile("/proc/sys/kernel/apparmor_restrict_unprivileged_userns"); err != nil || strings.TrimSpace(string(b)) != "1" {
		return p, true
	}
	if _, err := os.Stat("/etc/apparmor.d/gecko-desktop"); err == nil {
		return p, true
	}
	helper := filepath.Join(filepath.Dir(p), "..", "share", "gecko", "desktop", "node_modules", "electron", "dist", "chrome-sandbox")
	if st, err := os.Stat(helper); err == nil && st.Mode()&os.ModeSetuid != 0 && ownedByRoot(st) {
		return p, true
	}
	fmt.Fprintln(os.Stderr, "gecko: the desktop app's sandbox isn't set up on this system; using a browser window. Fix once with `make desktop-sandbox` in the gecko repo.")
	return "", false
}

// windowArgs places the app window where the last fitted one ended up:
// the size that gives an 80x24 terminal, centered on that screen. Without
// that (first launch) it's a close guess, and the page fixes it up.
func windowArgs() []string {
	w, h, x, y, ok := savedWindow()
	if !ok {
		return []string{"--window-size=900,500"}
	}
	return []string{fmt.Sprintf("--window-size=%d,%d", w, h), fmt.Sprintf("--window-position=%d,%d", x, y)}
}

// savedWindow is the size of the last fitted window, centered on its screen.
func savedWindow() (w, h, x, y int, ok bool) {
	var b struct{ W, H, SX, SY, SW, SH int }
	data, err := os.ReadFile(filepath.Join(config.StateDir(), "window.json"))
	if err != nil || json.Unmarshal(data, &b) != nil || b.W <= 0 || b.H <= 0 || b.SW <= 0 || b.SH <= 0 {
		return 0, 0, 0, 0, false
	}
	return b.W, b.H, b.SX + max(0, (b.SW-b.W)/2), b.SY + max(0, (b.SH-b.H)/2), true
}

// cmdOpen opens the app in a chromeless browser window when possible.
func cmdOpen() error {
	u, err := appURL()
	if err != nil {
		return err
	}
	// The page sizes the window so the terminal is exactly 80x24 and centers
	// it (it knows the font metrics); --window-size is a close first guess
	// to avoid a visible jump.
	u += "&fit=80x24"
	// The desktop app (`make desktop`) gives a native, see-through window.
	if p, ok := desktopApp(); ok {
		args := []string{"--url=" + u}
		if w, h, x, y, ok := savedWindow(); ok {
			args = append(args, fmt.Sprintf("--size=%d,%d", w, h), fmt.Sprintf("--position=%d,%d", x, y))
		}
		cmd := exec.Command(p, args...)
		cmd.Stdout, cmd.Stderr = nil, nil
		return cmd.Start()
	}
	app := "--app=" + u
	// Lets the startup chime play without a click first.
	flags := append(windowArgs(), "--autoplay-policy=no-user-gesture-required")
	switch runtime.GOOS {
	case "darwin":
		for _, b := range []string{"Google Chrome", "Microsoft Edge", "Brave Browser", "Chromium", "Arc"} {
			if _, err := os.Stat("/Applications/" + b + ".app"); err == nil {
				return exec.Command("open", append([]string{"-na", b, "--args", app}, flags...)...).Start()
			}
		}
		return exec.Command("open", u).Start()
	case "windows":
		if p := windowsBrowser(); p != "" {
			return exec.Command(p, append([]string{app}, flags...)...).Start()
		}
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", u).Start()
	default:
		for _, b := range []string{"google-chrome", "google-chrome-stable", "chromium", "chromium-browser", "microsoft-edge", "brave-browser"} {
			if p, err := exec.LookPath(b); err == nil {
				// --class lets the dock match the window to gecko.desktop.
				return exec.Command(p, append([]string{app, "--class=gecko"}, flags...)...).Start()
			}
		}
		return exec.Command("xdg-open", u).Start()
	}
}

// windowsBrowser finds a Chromium-family browser for the chromeless app
// window. Starting its exe directly, rather than through `cmd /c start`,
// keeps the `&` in the URL away from cmd.exe (which would cut the query
// string in two) and makes a failure to launch a real error.
func windowsBrowser() string {
	var roots []string
	for _, e := range []string{"LOCALAPPDATA", "ProgramFiles", "ProgramFiles(x86)"} {
		if v := os.Getenv(e); v != "" {
			roots = append(roots, v)
		}
	}
	for _, b := range []string{
		`Google\Chrome\Application\chrome.exe`,
		`Microsoft\Edge\Application\msedge.exe`,
		`BraveSoftware\Brave-Browser\Application\brave.exe`,
		`Chromium\Application\chrome.exe`,
	} {
		for _, root := range roots {
			if p := filepath.Join(root, b); fileExists(p) {
				return p
			}
		}
	}
	for _, b := range []string{"msedge.exe", "chrome.exe"} {
		if p, err := exec.LookPath(b); err == nil {
			return p
		}
	}
	return ""
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

func ago(ms int64) string {
	if ms <= 0 {
		return "-"
	}
	d := time.Since(time.UnixMilli(ms)).Round(time.Second)
	switch {
	case d < time.Minute:
		return d.String()
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 48*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	return fmt.Sprintf("%dd", int(d.Hours()/24))
}

func describe(s proto.SessionInfo) string {
	switch {
	case s.Agent != nil:
		return s.Agent.Name + " (" + s.Agent.Status + ")"
	case s.Kind == "ssh" || s.Kind == "mosh":
		return s.Kind + " " + s.Remote
	case s.Kind == "tmux" && s.TmuxSess != "":
		return "tmux " + s.TmuxSess
	case s.Command != "":
		return s.Command
	}
	return s.Shell
}

func cmdList(args []string) error {
	fs := flag.NewFlagSet("ls", flag.ExitOnError)
	asJSON := fs.Bool("json", false, "JSON output")
	_ = fs.Parse(args)
	c, err := client.Connect(true)
	if err != nil {
		return err
	}
	defer c.Close()
	ss, err := c.Sessions()
	if err != nil {
		return err
	}
	if *asJSON {
		return json.NewEncoder(os.Stdout).Encode(ss)
	}
	if len(ss) == 0 {
		fmt.Println("no sessions (create one with `gecko new`)")
		return nil
	}
	tw := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "ID\tWORKSPACE\tNAME\tRUNNING\tCWD\tACTIVE")
	for _, s := range ss {
		name := s.Name
		if name == "" {
			name = s.Title
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n", s.ID, s.Workspace, trunc(name, 24), trunc(describe(s), 40), trunc(s.Cwd, 40), ago(s.Activity))
	}
	return tw.Flush()
}

func trunc(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:n-1]) + "…"
	}
	return s
}

func cmdNew(args []string) error {
	fs := flag.NewFlagSet("new", flag.ExitOnError)
	ws := fs.String("w", "", "workspace")
	name := fs.String("n", "", "tab name")
	host := fs.String("H", "", "host (default: this machine)")
	cwd := fs.String("C", "", "working directory")
	att := fs.Bool("a", false, "attach after creating")
	execArgv := fs.Bool("exec", false, "exec the command directly instead of typing it into a shell")
	_ = fs.Parse(args)
	c, err := client.Connect(true)
	if err != nil {
		return err
	}
	defer c.Close()
	req := proto.CreateReq{Host: *host, Workspace: *ws, Name: *name, Cwd: *cwd}
	if *cwd == "" && *host == "" {
		req.Cwd, _ = os.Getwd()
	}
	if w, h, err := term.GetSize(int(os.Stdout.Fd())); err == nil {
		req.Cols, req.Rows = w, h
	}
	if rest := fs.Args(); len(rest) > 0 {
		if *execArgv {
			req.Argv = rest
		} else {
			req.Command = strings.Join(rest, " ")
		}
	}
	r, err := c.Call(&proto.Msg{T: proto.TCreate, Create: &req})
	if err != nil {
		return err
	}
	if *att {
		return attach.Run(c, r.Session.ID)
	}
	fmt.Println(r.Session.ID)
	return nil
}

func withSession(args []string, fn func(c *client.Client, id string) error) error {
	if len(args) != 1 {
		return fmt.Errorf("expected one session argument")
	}
	c, err := client.Connect(true)
	if err != nil {
		return err
	}
	defer c.Close()
	id, err := resolve(c, args[0])
	if err != nil {
		return err
	}
	return fn(c, id)
}

func resolve(c *client.Client, q string) (string, error) {
	ss, err := c.Sessions()
	if err != nil {
		return "", err
	}
	var match []string
	for _, s := range ss {
		lid := s.ID[strings.LastIndexByte(s.ID, '/')+1:]
		if s.ID == q || lid == q {
			return s.ID, nil
		}
		if s.Name == q || strings.HasPrefix(lid, q) || strings.HasPrefix(s.ID, q) {
			match = append(match, s.ID)
		}
	}
	if len(match) == 1 {
		return match[0], nil
	}
	if len(match) == 0 {
		return "", fmt.Errorf("no session matches %q", q)
	}
	return "", fmt.Errorf("%q is ambiguous: %s", q, strings.Join(match, ", "))
}

func cmdSend(args []string) error {
	fs := flag.NewFlagSet("send", flag.ExitOnError)
	noEnter := fs.Bool("no-enter", false, "don't press Enter after the text")
	_ = fs.Parse(args)
	if fs.NArg() < 2 {
		return fmt.Errorf("usage: gecko send <session> text...")
	}
	text := strings.Join(fs.Args()[1:], " ")
	if text == "-" {
		b, _ := io.ReadAll(os.Stdin)
		text = string(b)
	}
	return withSession(fs.Args()[:1], func(c *client.Client, id string) error {
		data := text
		if !*noEnter {
			data += "\r"
		}
		_, err := c.Call(&proto.Msg{T: proto.TInput, ID: id, Data: data})
		return err
	})
}

func cmdAgents(args []string) error {
	fs := flag.NewFlagSet("agents", flag.ExitOnError)
	asJSON := fs.Bool("json", false, "JSON output")
	_ = fs.Parse(args)
	c, err := client.Connect(true)
	if err != nil {
		return err
	}
	defer c.Close()
	ss, err := c.Sessions()
	if err != nil {
		return err
	}
	type row struct {
		Where string           `json:"where"`
		Host  string           `json:"host"`
		Agent proto.AgentState `json:"agent"`
		Cwd   string           `json:"cwd,omitempty"`
	}
	var rows []row
	for _, s := range ss {
		if s.Agent != nil {
			rows = append(rows, row{Where: s.ID, Host: s.Host, Agent: *s.Agent, Cwd: s.Cwd})
		}
	}
	if *asJSON {
		return json.NewEncoder(os.Stdout).Encode(rows)
	}
	if len(rows) == 0 {
		fmt.Println("no coding agents running")
		return nil
	}
	sort.SliceStable(rows, func(i, j int) bool { return rows[i].Agent.Status < rows[j].Agent.Status })
	tw := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "AGENT\tSTATUS\tFOR\tWHERE\tDETAIL")
	for _, r := range rows {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", r.Agent.Name, r.Agent.Status, ago(r.Agent.Since), r.Where, trunc(r.Agent.Detail, 60))
	}
	return tw.Flush()
}

// waitState opens a full client connection and returns the first state push.
func waitState() *proto.Msg {
	conn, err := daemon.Dial(false)
	if err != nil {
		return nil
	}
	defer conn.Close()
	_ = proto.WriteJSON(conn, &proto.Msg{T: proto.THello, Client: "cli-state"})
	deadline := time.AfterFunc(3*time.Second, func() { conn.Close() })
	defer deadline.Stop()
	for {
		bin, b, err := conn.Read()
		if err != nil {
			return nil
		}
		var m proto.Msg
		if !bin && json.Unmarshal(b, &m) == nil && m.T == proto.TState {
			return &m
		}
	}
}

func cmdWorkspace(args []string) error {
	if len(args) == 2 && args[0] == "open" {
		c, err := client.Connect(true)
		if err != nil {
			return err
		}
		defer c.Close()
		r, err := c.Call(&proto.Msg{T: proto.TOpenWS, Name: args[1]})
		if err != nil {
			return err
		}
		for _, s := range r.Sessions {
			fmt.Println(s.ID)
		}
		return nil
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	for _, w := range cfg.Workspaces {
		fmt.Printf("%s\t%s\t%s\t%d tabs\n", w.Name, w.Host, w.Cwd, len(w.Tabs))
	}
	if len(cfg.Workspaces) == 0 {
		fmt.Println("no workspace templates; add \"workspaces\" to", config.Path())
	}
	return nil
}

func cmdHost(args []string) error {
	sub := "ls"
	if len(args) > 0 {
		sub, args = args[0], args[1:]
	}
	switch sub {
	case "ls", "list":
		st := waitState()
		if st == nil {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			for _, h := range cfg.Hosts {
				fmt.Printf("%s\t%s\n", h.Name, h.SSH)
			}
			return nil
		}
		tw := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
		fmt.Fprintln(tw, "HOST\tSSH\tSTATUS\tOS")
		for _, h := range st.Hosts {
			target := h.Target
			if h.Local {
				target = "(this machine)"
			}
			status := h.Status
			if h.Error != "" {
				status += ": " + h.Error
			}
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", h.Name, target, status, h.OS)
		}
		return tw.Flush()
	case "add":
		if len(args) != 2 {
			return fmt.Errorf("usage: gecko host add <name> <ssh-destination>")
		}
		c, err := client.Connect(true)
		if err != nil {
			return err
		}
		defer c.Close()
		_, err = c.Call(&proto.Msg{T: proto.TAddHost, Name: args[0], Target: args[1]})
		if err == nil {
			fmt.Printf("added %s. If gecko isn't installed there yet: gecko host install %s\n", args[0], args[0])
		}
		return err
	case "rm", "remove":
		if len(args) != 1 {
			return fmt.Errorf("usage: gecko host rm <name>")
		}
		c, err := client.Connect(true)
		if err != nil {
			return err
		}
		defer c.Close()
		_, err = c.Call(&proto.Msg{T: proto.TRmHost, Name: args[0]})
		return err
	case "install":
		if len(args) != 1 {
			return fmt.Errorf("usage: gecko host install <name|ssh-destination>")
		}
		return hostInstall(args[0])
	}
	return fmt.Errorf("unknown host command %q", sub)
}

// hostInstall copies a matching gecko binary to the remote ~/.local/bin.
func hostInstall(name string) error {
	target := name
	if cfg, err := config.Load(); err == nil {
		for _, h := range cfg.Hosts {
			if h.Name == name && h.SSH != "" {
				target = h.SSH
			}
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	log := func(line string) { fmt.Println(line) }
	res, err := hostsetup.Probe(ctx, target, "", log)
	if err != nil {
		return err
	}
	return hostsetup.Install(ctx, target, res, log)
}

// cmdBridge pipes stdio to the local daemon socket; remote hosts run this
// over ssh. The daemon is started (detached) if necessary.
func cmdBridge() error {
	conn, err := daemon.Dial(true) // starts the daemon if needed
	if err != nil {
		return err
	}
	conn.Close()
	nc, err := net.Dial("unix", config.Socket())
	if err != nil {
		return err
	}
	done := make(chan struct{}, 2)
	go func() { _, _ = io.Copy(nc, os.Stdin); done <- struct{}{} }()
	go func() { _, _ = io.Copy(os.Stdout, nc); done <- struct{}{} }()
	<-done
	return nil
}

// cmdUpgrade restarts the daemon in place with the binary on disk. Shells,
// agents and their output survive; clients reconnect on their own.
func cmdUpgrade(args []string) error {
	fs := flag.NewFlagSet("upgrade", flag.ExitOnError)
	ifRunning := fs.Bool("if-running", false, "do nothing when no daemon is running")
	_ = fs.Parse(args)
	c, err := client.Connect(false)
	if err != nil {
		if *ifRunning {
			fmt.Printf("no Gecko service running in %s; nothing to restart\n", config.Home())
			return nil
		}
		return err
	}
	before := c.Start
	ss, _ := c.Sessions()
	_, err = c.Call(&proto.Msg{T: proto.TUpgrade})
	c.Close()
	if err != nil {
		if strings.Contains(err.Error(), "bad request") || strings.Contains(err.Error(), "doesn't know") {
			return fmt.Errorf("the running Gecko service is too old to restart in place; restart it once with `gecko stop && gecko` (this closes its sessions), after that upgrades keep them")
		}
		return err
	}
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		time.Sleep(150 * time.Millisecond)
		c, err := client.Connect(false)
		if err != nil {
			continue
		}
		after, kept := c.Start, 0
		if list, err := c.Sessions(); err == nil {
			kept = len(list)
		}
		c.Close()
		if after != before {
			fmt.Printf("gecko service restarted with %s; kept %d of %d sessions\n", version, kept, len(ss))
			return nil
		}
	}
	return errors.New("the service didn't restart; see " + daemon.LogPath())
}

func cmdStop() error {
	b, err := os.ReadFile(filepath.Join(config.StateDir(), "pid"))
	if err != nil && config.LegacySocket() != "" {
		b, err = os.ReadFile(filepath.Join(filepath.Dir(config.LegacySocket()), "pid"))
	}
	if err != nil {
		return fmt.Errorf("daemon not running")
	}
	var pid int
	fmt.Sscan(string(b), &pid)
	p, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	if runtime.GOOS == "windows" {
		return p.Kill()
	}
	return p.Signal(os.Interrupt)
}
