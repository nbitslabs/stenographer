package cmd

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/nbitslabs/stenographer/internal/service"
)

var serviceCmd = &cobra.Command{
	Use:   "service",
	Short: "Manage the Stenographer background service",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Service commands don't require config by default.
		// Individual subcommands load config when needed.
		return nil
	},
}

var serviceInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install Stenographer as a background service",
	RunE: func(cmd *cobra.Command, args []string) error {
		mgr, err := service.NewManager()
		if err != nil {
			return err
		}

		binaryPath, err := os.Executable()
		if err != nil {
			return fmt.Errorf("determine binary path: %w", err)
		}

		configPath, _ := filepath.Abs(cfgFile)

		if err := mgr.Install(binaryPath, configPath); err != nil {
			return fmt.Errorf("install service: %w", err)
		}

		fmt.Println("Service installed successfully. Use 'stenographer service start' to run.")
		return nil
	},
}

var serviceUninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Uninstall the Stenographer service",
	RunE: func(cmd *cobra.Command, args []string) error {
		mgr, err := service.NewManager()
		if err != nil {
			return err
		}

		if !mgr.IsInstalled() {
			fmt.Println("Service is not installed.")
			return nil
		}

		// Stop first if running.
		st, _ := mgr.Status()
		if st.Running {
			_ = mgr.Stop()
		}
		_ = mgr.Disable()

		if err := mgr.Uninstall(); err != nil {
			return fmt.Errorf("uninstall service: %w", err)
		}

		fmt.Println("Service uninstalled successfully.")
		return nil
	},
}

var serviceStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the Stenographer service",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Check session file exists (requires loading config).
		if err := loadConfigForService(); err != nil {
			return err
		}

		sessionPath := cfg.Telegram.SessionFile
		if _, err := os.Stat(sessionPath); os.IsNotExist(err) {
			return fmt.Errorf("no session found. Please run 'stenographer run' first to authenticate")
		}

		mgr, err := service.NewManager()
		if err != nil {
			return err
		}

		if err := mgr.Start(); err != nil {
			return fmt.Errorf("start service: %w", err)
		}

		fmt.Println("Service started successfully.")
		return nil
	},
}

var serviceStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the Stenographer service",
	RunE: func(cmd *cobra.Command, args []string) error {
		mgr, err := service.NewManager()
		if err != nil {
			return err
		}

		if err := mgr.Stop(); err != nil {
			return fmt.Errorf("stop service: %w", err)
		}

		fmt.Println("Service stopped successfully.")
		return nil
	},
}

var serviceRestartCmd = &cobra.Command{
	Use:   "restart",
	Short: "Restart the Stenographer service",
	RunE: func(cmd *cobra.Command, args []string) error {
		mgr, err := service.NewManager()
		if err != nil {
			return err
		}

		if err := mgr.Restart(); err != nil {
			return fmt.Errorf("restart service: %w", err)
		}

		fmt.Println("Service restarted successfully.")
		return nil
	},
}

var serviceEnableCmd = &cobra.Command{
	Use:   "enable",
	Short: "Enable auto-start on boot/login",
	RunE: func(cmd *cobra.Command, args []string) error {
		mgr, err := service.NewManager()
		if err != nil {
			return err
		}

		if !mgr.IsInstalled() {
			return fmt.Errorf("service is not installed. Run 'stenographer service install' first")
		}

		if err := mgr.Enable(); err != nil {
			return fmt.Errorf("enable service: %w", err)
		}

		fmt.Println("Service enabled for auto-start.")
		return nil
	},
}

var serviceDisableCmd = &cobra.Command{
	Use:   "disable",
	Short: "Disable auto-start",
	RunE: func(cmd *cobra.Command, args []string) error {
		mgr, err := service.NewManager()
		if err != nil {
			return err
		}

		if err := mgr.Disable(); err != nil {
			return fmt.Errorf("disable service: %w", err)
		}

		fmt.Println("Service auto-start disabled.")
		return nil
	},
}

var serviceStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check service status",
	RunE: func(cmd *cobra.Command, args []string) error {
		mgr, err := service.NewManager()
		if err != nil {
			return err
		}

		if !mgr.IsInstalled() {
			fmt.Println("Service is not installed")
			os.Exit(2)
		}

		st, err := mgr.Status()
		if err != nil {
			return fmt.Errorf("get status: %w", err)
		}

		if st.Running {
			fmt.Printf("Service is running (PID: %d)\n", st.PID)
			os.Exit(0)
		}

		fmt.Println("Service is not running")
		os.Exit(1)
		return nil
	},
}

var (
	logsLines  int
	logsFollow bool
)

var serviceLogsCmd = &cobra.Command{
	Use:   "logs",
	Short: "View service logs",
	RunE: func(cmd *cobra.Command, args []string) error {
		mgr, err := service.NewManager()
		if err != nil {
			return err
		}

		if runtime.GOOS == "linux" {
			return showJournalLogs(logsLines, logsFollow)
		}

		// macOS: read log file.
		logPath := mgr.LogPath()
		if logPath == "" {
			return fmt.Errorf("no log path available")
		}

		if logsFollow {
			tailCmd := exec.Command("tail", "-f", "-n", strconv.Itoa(logsLines), logPath)
			tailCmd.Stdout = os.Stdout
			tailCmd.Stderr = os.Stderr
			return tailCmd.Run()
		}

		return tailFile(logPath, logsLines)
	},
}

func showJournalLogs(lines int, follow bool) error {
	jargs := []string{"--user", "-u", "stenographer", "-n", strconv.Itoa(lines), "--no-pager"}
	if follow {
		jargs = append(jargs, "-f")
	}
	jcmd := exec.Command("journalctl", jargs...)
	jcmd.Stdout = os.Stdout
	jcmd.Stderr = os.Stderr
	return jcmd.Run()
}

func tailFile(path string, lines int) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open log file: %w", err)
	}
	defer f.Close()

	// Read all lines, then print the last N.
	var all []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		all = append(all, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return err
	}

	start := 0
	if len(all) > lines {
		start = len(all) - lines
	}
	for _, line := range all[start:] {
		fmt.Println(line)
	}
	return nil
}

func loadConfigForService() error {
	if cfg != nil {
		return nil
	}
	var err error
	cfg, err = loadConfig()
	return err
}

func init() {
	rootCmd.AddCommand(serviceCmd)
	serviceCmd.AddCommand(serviceInstallCmd)
	serviceCmd.AddCommand(serviceUninstallCmd)
	serviceCmd.AddCommand(serviceStartCmd)
	serviceCmd.AddCommand(serviceStopCmd)
	serviceCmd.AddCommand(serviceRestartCmd)
	serviceCmd.AddCommand(serviceEnableCmd)
	serviceCmd.AddCommand(serviceDisableCmd)
	serviceCmd.AddCommand(serviceStatusCmd)
	serviceCmd.AddCommand(serviceLogsCmd)

	serviceLogsCmd.Flags().IntVarP(&logsLines, "lines", "n", 100, "Number of log lines to show")
	serviceLogsCmd.Flags().BoolVarP(&logsFollow, "follow", "f", false, "Follow log output")
}
