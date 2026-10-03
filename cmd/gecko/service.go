package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// cmdService installs a login service so the daemon (and every session,
// agent included) is always there, independent of terminals and SSH.
func cmdService(args []string) error {
	if len(args) != 1 || args[0] != "install" {
		return fmt.Errorf("usage: gecko service install")
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	exe, _ = filepath.EvalSymlinks(exe)
	home, _ := os.UserHomeDir()
	switch runtime.GOOS {
	case "linux":
		dir := filepath.Join(home, ".config", "systemd", "user")
		unit := fmt.Sprintf(`[Unit]
Description=Gecko terminal daemon

[Service]
ExecStart=%s daemon
Restart=on-failure
KillMode=process

[Install]
WantedBy=default.target
`, exe)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(dir, "gecko.service"), []byte(unit), 0o644); err != nil {
			return err
		}
		_ = exec.Command("systemctl", "--user", "daemon-reload").Run()
		if out, err := exec.Command("systemctl", "--user", "enable", "gecko.service").CombinedOutput(); err != nil {
			return fmt.Errorf("systemctl: %s", out)
		}
		fmt.Println("installed ~/.config/systemd/user/gecko.service")
		fmt.Println("to keep sessions alive after you log out everywhere, run: loginctl enable-linger $USER")
		fmt.Println("(stop a running daemon with `gecko stop`, then `systemctl --user start gecko`)")
	case "darwin":
		p := filepath.Join(home, "Library", "LaunchAgents", "dev.gecko.daemon.plist")
		plist := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
  <key>Label</key><string>dev.gecko.daemon</string>
  <key>ProgramArguments</key><array><string>%s</string><string>daemon</string></array>
  <key>RunAtLoad</key><true/>
  <key>KeepAlive</key><dict><key>SuccessfulExit</key><false/></dict>
  <key>StandardErrorPath</key><string>%s/Library/Logs/gecko.log</string>
</dict></plist>
`, exe, home)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(p, []byte(plist), 0o644); err != nil {
			return err
		}
		_ = exec.Command("launchctl", "load", "-w", p).Run()
		fmt.Println("installed", p)
	case "windows":
		dir := filepath.Join(os.Getenv("APPDATA"), "Microsoft", "Windows", "Start Menu", "Programs", "Startup")
		vbs := fmt.Sprintf("CreateObject(\"WScript.Shell\").Run \"\"\"%s\"\" daemon\", 0, False\r\n", exe)
		p := filepath.Join(dir, "gecko.vbs")
		if err := os.WriteFile(p, []byte(vbs), 0o644); err != nil {
			return err
		}
		fmt.Println("installed", p)
	default:
		return fmt.Errorf("not supported on %s; run `gecko daemon` from your init system", runtime.GOOS)
	}
	return nil
}
