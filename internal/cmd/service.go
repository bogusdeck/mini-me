package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
)

const (
	LaunchdLabel  = "com.mini-me.daemon"
	PlistFileName = "com.mini-me.daemon.plist"
	SystemdUnit   = "mini-me.service"
)

var (
	flagServicePort int
	flagServiceHost string
)

var serviceCmd = &cobra.Command{
	Use:   "service",
	Short: "Manage mini-me background service (install, uninstall, start, stop, status)",
	Long:  `service manages mini-me as a background daemon (launchd on macOS, systemd on Linux).`,
}

var serviceInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install and start mini-me background service",
	RunE: func(cmd *cobra.Command, args []string) error {
		userHome, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed resolving home dir: %w", err)
		}

		execPath, err := os.Executable()
		if err != nil {
			return fmt.Errorf("failed resolving mini-me binary path: %w", err)
		}

		if runtime.GOOS == "linux" {
			systemdUserDir := filepath.Join(userHome, ".config", "systemd", "user")
			if err := os.MkdirAll(systemdUserDir, 0755); err != nil {
				return fmt.Errorf("failed creating systemd user dir: %w", err)
			}

			unitPath := filepath.Join(systemdUserDir, SystemdUnit)
			unitContent := fmt.Sprintf(`[Unit]
Description=mini-me local digital twin and RAG background service
After=network.target

[Service]
ExecStart=%s serve --host %s --port %d
Restart=always
RestartSec=5

[Install]
WantedBy=default.target
`, execPath, flagServiceHost, flagServicePort)

			if err := os.WriteFile(unitPath, []byte(unitContent), 0644); err != nil {
				return fmt.Errorf("failed writing systemd unit: %w", err)
			}

			_ = exec.Command("systemctl", "--user", "daemon-reload").Run()
			out, err := exec.Command("systemctl", "--user", "enable", "--now", SystemdUnit).CombinedOutput()
			if err != nil {
				return fmt.Errorf("failed enabling systemd service: %s (%w)", string(out), err)
			}

			fmt.Println("[OK] mini-me systemd user service installed and started successfully!")
			fmt.Printf("  Binary:    %s\n", execPath)
			fmt.Printf("  Unit:      %s\n", unitPath)
			fmt.Printf("  Endpoint:  http://%s:%d\n", flagServiceHost, flagServicePort)
			return nil
		}

		// Default to macOS launchd
		launchAgentsDir := filepath.Join(userHome, "Library", "LaunchAgents")
		if err := os.MkdirAll(launchAgentsDir, 0755); err != nil {
			return fmt.Errorf("failed creating LaunchAgents dir: %w", err)
		}

		plistPath := filepath.Join(launchAgentsDir, PlistFileName)
		logDir := filepath.Join(userHome, "Library", "Logs")
		_ = os.MkdirAll(logDir, 0755)

		logPath := filepath.Join(logDir, "mini-me.log")
		errLogPath := filepath.Join(logDir, "mini-me.error.log")

		plistContent := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>%s</string>
    <key>ProgramArguments</key>
    <array>
        <string>%s</string>
        <string>serve</string>
        <string>--host</string>
        <string>%s</string>
        <string>--port</string>
        <string>%d</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>%s</string>
    <key>StandardErrorPath</key>
    <string>%s</string>
</dict>
</plist>`, LaunchdLabel, execPath, flagServiceHost, flagServicePort, logPath, errLogPath)

		if err := os.WriteFile(plistPath, []byte(plistContent), 0644); err != nil {
			return fmt.Errorf("failed writing plist file: %w", err)
		}

		_ = exec.Command("launchctl", "unload", plistPath).Run()
		out, err := exec.Command("launchctl", "load", plistPath).CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed loading service with launchctl: %s (%w)", string(out), err)
		}

		fmt.Println("[OK] mini-me service installed and started successfully!")
		fmt.Printf("  Binary:    %s\n", execPath)
		fmt.Printf("  Plist:     %s\n", plistPath)
		fmt.Printf("  Endpoint:  http://%s:%d\n", flagServiceHost, flagServicePort)
		fmt.Printf("  Logs:      %s\n", logPath)

		return nil
	},
}

var serviceUninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Stop and uninstall mini-me background service",
	RunE: func(cmd *cobra.Command, args []string) error {
		userHome, err := os.UserHomeDir()
		if err != nil {
			return err
		}

		if runtime.GOOS == "linux" {
			_ = exec.Command("systemctl", "--user", "disable", "--now", SystemdUnit).Run()
			unitPath := filepath.Join(userHome, ".config", "systemd", "user", SystemdUnit)
			_ = os.Remove(unitPath)
			_ = exec.Command("systemctl", "--user", "daemon-reload").Run()
			fmt.Println("[OK] mini-me systemd service uninstalled successfully.")
			return nil
		}

		plistPath := filepath.Join(userHome, "Library", "LaunchAgents", PlistFileName)
		_ = exec.Command("launchctl", "unload", plistPath).Run()
		_ = os.Remove(plistPath)

		fmt.Println("[OK] mini-me service uninstalled successfully.")
		return nil
	},
}

var serviceStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the mini-me background service",
	RunE: func(cmd *cobra.Command, args []string) error {
		if runtime.GOOS == "linux" {
			out, err := exec.Command("systemctl", "--user", "start", SystemdUnit).CombinedOutput()
			if err != nil {
				return fmt.Errorf("failed starting systemd service: %s (%w)", string(out), err)
			}
			fmt.Println("[OK] mini-me service started.")
			return nil
		}

		userHome, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		plistPath := filepath.Join(userHome, "Library", "LaunchAgents", PlistFileName)
		out, err := exec.Command("launchctl", "load", plistPath).CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed starting service: %s (%w)", string(out), err)
		}
		fmt.Println("[OK] mini-me service started.")
		return nil
	},
}

var serviceStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the mini-me background service",
	RunE: func(cmd *cobra.Command, args []string) error {
		if runtime.GOOS == "linux" {
			out, err := exec.Command("systemctl", "--user", "stop", SystemdUnit).CombinedOutput()
			if err != nil {
				return fmt.Errorf("failed stopping systemd service: %s (%w)", string(out), err)
			}
			fmt.Println("[OK] mini-me service stopped.")
			return nil
		}

		userHome, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		plistPath := filepath.Join(userHome, "Library", "LaunchAgents", PlistFileName)
		out, err := exec.Command("launchctl", "unload", plistPath).CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed stopping service: %s (%w)", string(out), err)
		}
		fmt.Println("[OK] mini-me service stopped.")
		return nil
	},
}

var serviceStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check background service status",
	RunE: func(cmd *cobra.Command, args []string) error {
		if runtime.GOOS == "linux" {
			out, err := exec.Command("systemctl", "--user", "status", SystemdUnit).CombinedOutput()
			outputStr := string(out)
			if err != nil && !strings.Contains(outputStr, "active (running)") {
				fmt.Println("Status: INACTIVE (systemd service is not active)")
				fmt.Println("Run 'mini-me service install' to activate.")
				return nil
			}
			fmt.Println("Status: ACTIVE (systemd background service is running)")
			fmt.Println(outputStr)
			return nil
		}

		out, err := exec.Command("launchctl", "list", LaunchdLabel).CombinedOutput()
		outputStr := string(out)

		if err != nil || strings.Contains(outputStr, "Could not find service") {
			fmt.Println("Status: INACTIVE (service is not installed or not running)")
			fmt.Println("Run 'mini-me service install' or 'brew services start mini-me' to activate.")
			return nil
		}

		fmt.Println("Status: ACTIVE (mini-me background daemon is running)")
		fmt.Println(outputStr)
		return nil
	},
}

func init() {
	serviceInstallCmd.Flags().StringVar(&flagServiceHost, "host", "127.0.0.1", "bind host for background HTTP API")
	serviceInstallCmd.Flags().IntVar(&flagServicePort, "port", 8080, "bind port for background HTTP API")

	serviceCmd.AddCommand(serviceInstallCmd, serviceUninstallCmd, serviceStartCmd, serviceStopCmd, serviceStatusCmd)
	RootCmd.AddCommand(serviceCmd)
}
