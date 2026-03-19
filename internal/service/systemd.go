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

const serviceName = "stenographer"

var systemdUnitTemplate = template.Must(template.New("unit").Parse(`[Unit]
Description=Stenographer Telegram message logger
After=network-online.target
Wants=network-online.target

[Service]
ExecStart={{.BinaryPath}} run --config {{.ConfigPath}}
WorkingDirectory={{.HomeDir}}
Restart=on-failure
RestartSec=10s

[Install]
WantedBy=default.target
`))

// SystemdManager implements Manager for Linux systemd user services.
type SystemdManager struct {
	unitPath string
}

// NewSystemdManager creates a systemd service manager.
func NewSystemdManager() *SystemdManager {
	home, _ := os.UserHomeDir()
	unitDir := filepath.Join(home, ".config", "systemd", "user")
	return &SystemdManager{
		unitPath: filepath.Join(unitDir, serviceName+".service"),
	}
}

func (m *SystemdManager) Install(binaryPath, configPath string) error {
	dir := filepath.Dir(m.unitPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create systemd user dir: %w", err)
	}

	home, _ := os.UserHomeDir()
	data := map[string]string{
		"BinaryPath": binaryPath,
		"ConfigPath": configPath,
		"HomeDir":    home,
	}

	f, err := os.Create(m.unitPath)
	if err != nil {
		return fmt.Errorf("create unit file: %w", err)
	}
	defer f.Close()

	if err := systemdUnitTemplate.Execute(f, data); err != nil {
		return fmt.Errorf("write unit file: %w", err)
	}

	return systemctl("daemon-reload")
}

func (m *SystemdManager) Uninstall() error {
	if err := os.Remove(m.unitPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove unit file: %w", err)
	}
	return systemctl("daemon-reload")
}

func (m *SystemdManager) Start() error   { return systemctl("start", serviceName) }
func (m *SystemdManager) Stop() error    { return systemctl("stop", serviceName) }
func (m *SystemdManager) Restart() error { return systemctl("restart", serviceName) }
func (m *SystemdManager) Enable() error  { return systemctl("enable", serviceName) }
func (m *SystemdManager) Disable() error { return systemctl("disable", serviceName) }

func (m *SystemdManager) Status() (ServiceStatus, error) {
	s := ServiceStatus{Installed: m.IsInstalled()}
	if !s.Installed {
		return s, nil
	}

	out, err := exec.Command("systemctl", "--user", "show", serviceName,
		"--property=ActiveState,MainPID").Output()
	if err != nil {
		return s, nil
	}

	for _, line := range strings.Split(string(out), "\n") {
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		switch parts[0] {
		case "ActiveState":
			s.Running = parts[1] == "active"
		case "MainPID":
			s.PID, _ = strconv.Atoi(parts[1])
		}
	}

	return s, nil
}

func (m *SystemdManager) IsInstalled() bool {
	_, err := os.Stat(m.unitPath)
	return err == nil
}

func (m *SystemdManager) LogPath() string {
	return "" // systemd uses journalctl
}

func systemctl(args ...string) error {
	cmdArgs := append([]string{"--user"}, args...)
	cmd := exec.Command("systemctl", cmdArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
