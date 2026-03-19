package service

import (
	"fmt"
	"runtime"
)

// Manager defines the interface for platform-specific service management.
type Manager interface {
	// Install writes the service file to the platform-specific location.
	Install(binaryPath, configPath string) error
	// Uninstall removes the service file.
	Uninstall() error
	// Start starts the service.
	Start() error
	// Stop stops the service.
	Stop() error
	// Restart restarts the service.
	Restart() error
	// Enable enables auto-start on boot/login.
	Enable() error
	// Disable disables auto-start.
	Disable() error
	// Status returns the service status.
	Status() (ServiceStatus, error)
	// IsInstalled checks if the service file exists.
	IsInstalled() bool
	// LogPath returns the path to the log file (macOS) or empty string (Linux uses journalctl).
	LogPath() string
}

// ServiceStatus represents the current state of the service.
type ServiceStatus struct {
	Running   bool
	PID       int
	Installed bool
}

// NewManager returns a platform-specific service manager.
func NewManager() (Manager, error) {
	switch runtime.GOOS {
	case "linux":
		return NewSystemdManager(), nil
	case "darwin":
		return NewLaunchctlManager(), nil
	default:
		return nil, fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
}
