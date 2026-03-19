package service

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"
)

const plistLabel = "com.stenographer.agent"

var plistTemplate = template.Must(template.New("plist").Parse(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>{{.Label}}</string>
    <key>ProgramArguments</key>
    <array>
        <string>{{.BinaryPath}}</string>
        <string>run</string>
        <string>--config</string>
        <string>{{.ConfigPath}}</string>
    </array>
    <key>KeepAlive</key>
    <true/>
    <key>RunAtLoad</key>
    <true/>
    <key>StandardOutPath</key>
    <string>{{.LogDir}}/stenographer.out.log</string>
    <key>StandardErrorPath</key>
    <string>{{.LogDir}}/stenographer.err.log</string>
</dict>
</plist>
`))

// LaunchctlManager implements Manager for macOS launchctl.
type LaunchctlManager struct {
	plistPath string
	logDir    string
}

// NewLaunchctlManager creates a launchctl service manager.
func NewLaunchctlManager() *LaunchctlManager {
	home, _ := os.UserHomeDir()
	return &LaunchctlManager{
		plistPath: filepath.Join(home, "Library", "LaunchAgents", plistLabel+".plist"),
		logDir:    filepath.Join(home, ".config", "stenographer", "logs"),
	}
}

func (m *LaunchctlManager) Install(binaryPath, configPath string) error {
	dir := filepath.Dir(m.plistPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create LaunchAgents dir: %w", err)
	}

	if err := os.MkdirAll(m.logDir, 0700); err != nil {
		return fmt.Errorf("create log dir: %w", err)
	}

	data := map[string]string{
		"Label":      plistLabel,
		"BinaryPath": binaryPath,
		"ConfigPath": configPath,
		"LogDir":     m.logDir,
	}

	f, err := os.Create(m.plistPath)
	if err != nil {
		return fmt.Errorf("create plist file: %w", err)
	}
	defer f.Close()

	return plistTemplate.Execute(f, data)
}

func (m *LaunchctlManager) Uninstall() error {
	if err := os.Remove(m.plistPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove plist file: %w", err)
	}
	return nil
}

func (m *LaunchctlManager) Start() error {
	return exec.Command("launchctl", "load", m.plistPath).Run()
}

func (m *LaunchctlManager) Stop() error {
	return exec.Command("launchctl", "unload", m.plistPath).Run()
}

func (m *LaunchctlManager) Restart() error {
	_ = m.Stop()
	return m.Start()
}

// Enable is a no-op on macOS: RunAtLoad=true in the plist handles auto-start.
func (m *LaunchctlManager) Enable() error { return nil }

// Disable removes RunAtLoad by unloading the plist.
func (m *LaunchctlManager) Disable() error { return m.Stop() }

func (m *LaunchctlManager) Status() (ServiceStatus, error) {
	s := ServiceStatus{Installed: m.IsInstalled()}
	if !s.Installed {
		return s, nil
	}

	out, err := exec.Command("launchctl", "list").Output()
	if err != nil {
		return s, nil
	}

	for _, line := range strings.Split(string(out), "\n") {
		if strings.Contains(line, plistLabel) {
			fields := strings.Fields(line)
			if len(fields) >= 1 {
				pid, _ := strconv.Atoi(fields[0])
				if pid > 0 {
					s.Running = true
					s.PID = pid
				}
			}
			break
		}
	}

	return s, nil
}

func (m *LaunchctlManager) IsInstalled() bool {
	_, err := os.Stat(m.plistPath)
	return err == nil
}

func (m *LaunchctlManager) LogPath() string {
	return filepath.Join(m.logDir, "stenographer.out.log")
}
