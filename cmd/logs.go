package cmd

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

var (
	logLevel  string
	logFilter string
	logApp    string
)

var logsCmd = &cobra.Command{
	Use:     "logs [device-name-or-udid]",
	Aliases: []string{"log"},
	Short:   "Stream live logs from a device",
	Long: `Stream logs from a running iOS simulator or Android emulator in real-time.
	
If no device is specified, the first booted device is used automatically.`,
	ValidArgsFunction: validDeviceArgs,
	Args:              cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var deviceID string
		if len(args) == 0 {
			selected, err := PromptDeviceSelector("booted")
			if err != nil {
				return err
			}
			deviceID = selected
		} else {
			deviceID = args[0]
		}

		return streamLogs(deviceID, logLevel, logFilter, logApp)
	},
}

func init() {
	logsCmd.Flags().StringVarP(&logLevel, "level", "l", "info", "Filter by log level (debug, info, warn, error)")
	logsCmd.Flags().StringVarP(&logFilter, "filter", "f", "", "Grep pattern to filter logs in real-time")
	logsCmd.Flags().StringVarP(&logApp, "app", "a", "", "Filter by app bundle ID (iOS) or package name (Android)")
}

//nolint:gocyclo,cyclop
func streamLogs(deviceID, level, filter, app string) error {
	udid, name, isAndroid, err := FindRunningDevice(deviceID)
	if err != nil {
		return err
	}
	PrintInfo(fmt.Sprintf("Streaming logs from '%s' (Press Ctrl+C to stop)...", name))

	var logCmd *exec.Cmd

	if isAndroid {
		// Android logcat command
		args := []string{"-s", udid, "logcat"}
		if level != "" {
			lvl := "I"
			switch level {
			case "debug":
				lvl = "D"
			case "info":
				lvl = "I"
			case "warn":
				lvl = "W"
			case "error":
				lvl = "E"
			}
			args = append(args, "*:"+lvl)
		}

		if app != "" {
			// Get PID for the app
			pidOut, _ := packageExecutor.Output(CmdAdb, "-s", udid, "shell", "pidof", app)
			pid := strings.TrimSpace(string(pidOut))
			if pid != "" {
				args = append(args, "--pid="+pid)
			} else {
				PrintInfo(fmt.Sprintf("Warning: App '%s' not found or not running. Logs may be empty.", app))
			}
		}

		if filter != "" {
			// Need to use bash to pipe through grep
			commandStr := fmt.Sprintf("%s %s | grep --line-buffered '%s'", CmdAdb, strings.Join(args, " "), filter)
			logCmd = exec.Command("sh", "-c", commandStr)
		} else {
			logCmd = exec.Command(CmdAdb, args...)
		}
	} else {
		// iOS log stream command
		args := []string{"simctl", "spawn", udid, "log", "stream"}

		if level != "" {
			args = append(args, "--level", level)
		}

		if app != "" {
			args = append(args, "--predicate", fmt.Sprintf("subsystem == \"%s\"", app))
		}

		if filter != "" {
			commandStr := fmt.Sprintf("%s %s | grep --line-buffered '%s'", CmdXCrun, strings.Join(args, " "), filter)
			logCmd = exec.Command("sh", "-c", commandStr)
		} else {
			logCmd = exec.Command(CmdXCrun, args...)
		}
	}

	stdout, err := logCmd.StdoutPipe()
	if err != nil {
		return err
	}
	logCmd.Stderr = logCmd.Stdout // merge stderr into stdout for parsing

	if err := logCmd.Start(); err != nil {
		return fmt.Errorf("failed to start log stream: %w", err)
	}

	err = runLogViewer(stdout)

	// Ensure the log command is killed when bubbletea exits
	if logCmd.Process != nil {
		_ = logCmd.Process.Kill()
	}

	return err
}
