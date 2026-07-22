package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
)

// ---------------------------------------------------------------------------
// Paths & helpers
// ---------------------------------------------------------------------------

// camMscDir is an optional override for the MiniSimCam directory.
var camMscDir string

func miniSimCamDir() string {
	if camMscDir != "" {
		return camMscDir
	}
	ex, _ := os.Executable()
	// Walk up to the project root where MiniSimCam/ lives.
	dir := filepath.Dir(ex)
	for i := 0; i < 4; i++ {
		candidate := filepath.Join(dir, "MiniSimCam")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
		dir = filepath.Dir(dir)
	}
	// Fall back to CWD/MiniSimCam.
	cwd, _ := os.Getwd()
	return filepath.Join(cwd, "MiniSimCam")
}

func shmPath(udid string) string         { return fmt.Sprintf("/tmp/minisimcam.%s.frames", udid) }
func statusFilePath(udid string) string  { return fmt.Sprintf("/tmp/minisimcam.%s.status", udid) }
func pidFilePath(udid string) string     { return fmt.Sprintf("/tmp/minisimcam.%s.pid", udid) }
func frameHostBin(mscDir string) string  { return filepath.Join(mscDir, ".build", "release", "FrameHost") }
func injectorDylib(mscDir string) string { return filepath.Join(mscDir, ".build", "injector", "MiniCamInject.dylib") }
func buildScript(mscDir string) string   { return filepath.Join(mscDir, "Scripts", "build.sh") }

// ---------------------------------------------------------------------------
// cam command group
// ---------------------------------------------------------------------------

var camCmd = &cobra.Command{
	Use:   "cam",
	Short: "iOS Simulator camera injector",
	Long: `MiniSimCam — inject synthetic camera frames into an iOS Simulator app.

The cam group manages a macOS FrameHost process that writes BGRA frames to
shared memory. A dylib (MiniCamInject) loaded into your simulator app reads
those frames and delivers them as CMSampleBuffer callbacks.

Quick start:
  sim cam build
  sim cam start --image test.png
  sim cam launch --bundle-id com.example.MyApp
  sim cam status
  sim cam stop`,
}

// ---------------------------------------------------------------------------
// sim cam build
// ---------------------------------------------------------------------------

var camBuildCmd = &cobra.Command{
	Use:   "build",
	Short: "Build FrameHost and MiniCamInject.dylib",
	Long:  `Compiles the Swift FrameHost binary and the iOS Simulator injector dylib.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if runtime.GOOS != DarwinOS {
			return fmt.Errorf("cam build is only supported on macOS")
		}
		mscDir := miniSimCamDir()
		script := buildScript(mscDir)
		if _, err := os.Stat(script); err != nil {
			return fmt.Errorf("build script not found at %s — is MiniSimCam/ present?", script)
		}

		return RunSpinner("Building MiniSimCam (FrameHost + MiniCamInject)…", func() error {
			c := exec.Command("/bin/bash", script, mscDir)
			c.Stdout = os.Stdout
			c.Stderr = os.Stderr
			return c.Run()
		})
	},
}

// ---------------------------------------------------------------------------
// sim cam start
// ---------------------------------------------------------------------------

var (
	camStartImage  string
	camStartBars   bool
	camStartCamera bool
	camStartWidth  int
	camStartHeight int
	camStartFPS    int
	camStartDevice string
)

var camStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the FrameHost (frame producer)",
	Long: `Start the FrameHost macOS process that writes BGRA frames to shared memory.

Examples:
  sim cam start --image ./test-card.png
  sim cam start --bars --width 1920 --height 1080 --fps 60
  sim cam start --image qr.png --device <UDID>`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if runtime.GOOS != DarwinOS {
			return fmt.Errorf("cam start is only supported on macOS")
		}
		if camStartImage == "" && !camStartBars && !camStartCamera {
			return fmt.Errorf("provide --image <path>, --bars, or --camera")
		}

		// Resolve UDID.
		udid, _, _, err := FindRunningDevice(camStartDevice)
		if err != nil || udid == "" {
			return fmt.Errorf("no booted iOS simulator found — boot one first (%w)", err)
		}

		mscDir := miniSimCamDir()
		bin := frameHostBin(mscDir)
		if _, err := os.Stat(bin); err != nil {
			return fmt.Errorf("FrameHost not built — run 'sim cam build' first")
		}

		// Kill any existing FrameHost for this UDID.
		_ = stopFrameHost(udid)

		hostArgs := []string{
			"--udid", udid,
			"--width", strconv.Itoa(camStartWidth),
			"--height", strconv.Itoa(camStartHeight),
			"--fps", strconv.Itoa(camStartFPS),
		}
		if camStartBars {
			hostArgs = append(hostArgs, "--bars")
		} else if camStartCamera {
			hostArgs = append(hostArgs, "--camera")
		} else {
			absImage, err := filepath.Abs(camStartImage)
			if err != nil {
				return fmt.Errorf("cannot resolve image path: %w", err)
			}
			hostArgs = append(hostArgs, "--image", absImage)
		}

		c := exec.Command(bin, hostArgs...)
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr
		// Detach from the terminal so FrameHost survives terminal close,
		// EXCEPT when using the camera, as detaching breaks TCC (camera) permission inheritance.
		if !camStartCamera {
			c.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
		}
		if err := c.Start(); err != nil {
			return fmt.Errorf("failed to start FrameHost: %w", err)
		}

		// Give it a moment to write the PID file before returning.
		time.Sleep(500 * time.Millisecond)

		source := camStartImage
		if camStartBars {
			source = "color-bars"
		} else if camStartCamera {
			source = "mac-camera"
		}
		PrintSuccess(fmt.Sprintf(
			"FrameHost started — source=%s %dx%d @ %d fps (simulator %s)",
			source, camStartWidth, camStartHeight, camStartFPS, udid,
		))
		return nil
	},
}

// ---------------------------------------------------------------------------
// sim cam launch
// ---------------------------------------------------------------------------

var (
	camLaunchBundle string
	camLaunchDevice string
)

var camLaunchCmd = &cobra.Command{
	Use:   "launch",
	Short: "Launch an app on the simulator with MiniCamInject loaded",
	Long: `Launches your app on the booted iOS Simulator with MiniCamInject.dylib
injected via SIMCTL_CHILD_DYLD_INSERT_LIBRARIES.

The FrameHost must be running first (sim cam start).

Example:
  sim cam launch --bundle-id com.example.CameraPreviewApp`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if runtime.GOOS != DarwinOS {
			return fmt.Errorf("cam launch is only supported on macOS")
		}
		if camLaunchBundle == "" {
			return fmt.Errorf("--bundle-id is required")
		}

		udid, _, _, err := FindRunningDevice(camLaunchDevice)
		if err != nil || udid == "" {
			return fmt.Errorf("no booted iOS simulator found (%w)", err)
		}

		mscDir := miniSimCamDir()
		dylib := injectorDylib(mscDir)
		if _, err := os.Stat(dylib); err != nil {
			return fmt.Errorf("MiniCamInject.dylib not found — run 'sim cam build' first")
		}

		shm := shmPath(udid)

		PrintInfo(fmt.Sprintf("Launching %s on %s", camLaunchBundle, udid))
		PrintInfo(fmt.Sprintf("  dylib: %s", dylib))
		PrintInfo(fmt.Sprintf("  shm:   %s", shm))

		// Ensure get-task-allow entitlement is present so DYLD_INSERT_LIBRARIES works.
		appPathBytes, err := exec.Command("xcrun", "simctl", "get_app_container", udid, camLaunchBundle, "app").Output()
		if err == nil {
			appPath := strings.TrimSpace(string(appPathBytes))
			entOut, _ := exec.Command("codesign", "-d", "--entitlements", ":-", appPath).Output()
			entXML := string(entOut)
			
			if !strings.Contains(entXML, "com.apple.security.get-task-allow") {
				PrintInfo("  Injecting get-task-allow entitlement to permit dylib injection...")
				if strings.Contains(entXML, "<dict>") {
					entXML = strings.Replace(entXML, "<dict>", "<dict>\n\t<key>com.apple.security.get-task-allow</key>\n\t<true/>", 1)
				} else {
					entXML = `<?xml version="1.0" encoding="UTF-8"?><!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd"><plist version="1.0"><dict><key>com.apple.security.get-task-allow</key><true/></dict></plist>`
				}
				entPath := filepath.Join(os.TempDir(), "minisimcam_entitlements.plist")
				if writeErr := os.WriteFile(entPath, []byte(entXML), 0644); writeErr != nil {
					PrintInfo(fmt.Sprintf("  Warning: cannot write entitlements file: %v", writeErr))
				} else if signErr := exec.Command("codesign", "-f", "-s", "-", "--entitlements", entPath, appPath).Run(); signErr != nil {
					PrintInfo(fmt.Sprintf("  Warning: codesign re-sign failed: %v", signErr))
				} else {
					PrintInfo("  Re-signed with get-task-allow.")
				}
			}
		}

		c := exec.Command("xcrun", "simctl", "launch", udid, camLaunchBundle)
		c.Env = append(os.Environ(),
			"SIMCTL_CHILD_DYLD_INSERT_LIBRARIES="+dylib,
			"SIMCTL_CHILD_MINISIMCAM_PATH="+shm,
		)
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr
		if err := c.Run(); err != nil {
			return fmt.Errorf("simctl launch failed: %w", err)
		}

		PrintSuccess(fmt.Sprintf("Launched %s with MiniCamInject", camLaunchBundle))
		return nil
	},
}

// ---------------------------------------------------------------------------
// sim cam status
// ---------------------------------------------------------------------------

var camStatusDevice string

// camFrameLoopStatus mirrors FrameLoopStatus defined in FrameLoop.swift.
type camFrameLoopStatus struct {
	UDID           string  `json:"udid"`
	Source         string  `json:"source"`
	Width          int     `json:"width"`
	Height         int     `json:"height"`
	FPS            int     `json:"fps"`
	FramesProduced uint64  `json:"framesProduced"`
	HostPID        int32   `json:"hostPID"`
	StartedAt      string  `json:"startedAt"`
	LastFrameAgeMs float64 `json:"lastFrameAgeMs"`
	Running        bool    `json:"running"`
}

var camStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show FrameHost status and frame statistics",
	RunE: func(cmd *cobra.Command, args []string) error {
		if runtime.GOOS != DarwinOS {
			return fmt.Errorf("cam status is only supported on macOS")
		}

		udid, name, _, err := FindRunningDevice(camStatusDevice)
		if err != nil || udid == "" {
			return fmt.Errorf("no booted iOS simulator found (%w)", err)
		}

		statusPath := statusFilePath(udid)
		data, err := os.ReadFile(statusPath)
		if err != nil {
			PrintInfo(fmt.Sprintf("No status file found at %s", statusPath))
			PrintInfo("Is 'sim cam start' running?")
			return nil
		}

		var st camFrameLoopStatus
		if err := json.Unmarshal(data, &st); err != nil {
			return fmt.Errorf("cannot parse status file: %w", err)
		}

		lines := []string{
			fmt.Sprintf("Simulator:       %s (%s)", name, udid),
			fmt.Sprintf("Source:          %s", st.Source),
			fmt.Sprintf("Resolution:      %dx%d BGRA", st.Width, st.Height),
			fmt.Sprintf("Frame rate:      %d fps", st.FPS),
			fmt.Sprintf("Frames produced: %d", st.FramesProduced),
			fmt.Sprintf("Last frame age:  %.0f ms", st.LastFrameAgeMs),
			fmt.Sprintf("Host PID:        %d", st.HostPID),
			fmt.Sprintf("Started at:      %s", st.StartedAt),
			fmt.Sprintf("Running:         %v", st.Running),
		}

		width := 0
		for _, l := range lines {
			if len(l) > width {
				width = len(l)
			}
		}

		border := strings.Repeat("─", width+4)
		fmt.Println("┌" + border + "┐")
		fmt.Println("│  MiniSimCam Status" + strings.Repeat(" ", width+4-len("  MiniSimCam Status")) + "│")
		fmt.Println("├" + border + "┤")
		for _, l := range lines {
			fmt.Printf("│  %-*s  │\n", width, l)
		}
		fmt.Println("└" + border + "┘")
		return nil
	},
}

// ---------------------------------------------------------------------------
// sim cam stop
// ---------------------------------------------------------------------------

var camStopDevice string

var camStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the FrameHost process",
	RunE: func(cmd *cobra.Command, args []string) error {
		if runtime.GOOS != DarwinOS {
			return fmt.Errorf("cam stop is only supported on macOS")
		}

		udid, _, _, err := FindRunningDevice(camStopDevice)
		if err != nil || udid == "" {
			return fmt.Errorf("no booted iOS simulator found (%w)", err)
		}

		if err := stopFrameHost(udid); err != nil {
			return err
		}
		PrintSuccess("FrameHost stopped.")
		return nil
	},
}

// stopFrameHost reads the PID file and sends SIGTERM.
func stopFrameHost(udid string) error {
	pidPath := pidFilePath(udid)
	data, err := os.ReadFile(pidPath)
	if err != nil {
		return nil // No PID file — nothing to stop.
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return fmt.Errorf("invalid PID in %s: %w", pidPath, err)
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return nil
	}
	_ = proc.Signal(os.Interrupt) // SIGINT — triggers clean shutdown.
	return nil
}

// ---------------------------------------------------------------------------
// init: register sub-commands and flags
// ---------------------------------------------------------------------------

func init() {
	// cam-level flag: --msc-dir override
	camCmd.PersistentFlags().StringVar(&camMscDir, "msc-dir", "", "Path to MiniSimCam directory (default: auto-detected)")

	// build
	camCmd.AddCommand(camBuildCmd)

	// start
	camStartCmd.Flags().StringVar(&camStartImage, "image", "", "Path to PNG or JPEG source image")
	camStartCmd.Flags().BoolVar(&camStartBars, "bars", false, "Use synthetic SMPTE color-bar image")
	camStartCmd.Flags().BoolVar(&camStartCamera, "camera", false, "Use the Mac's physical camera as a live source")
	camStartCmd.Flags().IntVar(&camStartWidth, "width", DefaultCamWidth, "Frame width in pixels")
	camStartCmd.Flags().IntVar(&camStartHeight, "height", DefaultCamHeight, "Frame height in pixels")
	camStartCmd.Flags().IntVar(&camStartFPS, "fps", DefaultCamFPS, "Frames per second (1-120)")
	camStartCmd.Flags().StringVar(&camStartDevice, "device", "", "Simulator name or UDID (default: booted)")
	camCmd.AddCommand(camStartCmd)

	// launch
	camLaunchCmd.Flags().StringVar(&camLaunchBundle, "bundle-id", "", "App bundle identifier (required)")
	camLaunchCmd.Flags().StringVar(&camLaunchDevice, "device", "", "Simulator name or UDID (default: booted)")
	_ = camLaunchCmd.MarkFlagRequired("bundle-id")
	camCmd.AddCommand(camLaunchCmd)

	// status
	camStatusCmd.Flags().StringVar(&camStatusDevice, "device", "", "Simulator name or UDID (default: booted)")
	camCmd.AddCommand(camStatusCmd)

	// stop
	camStopCmd.Flags().StringVar(&camStopDevice, "device", "", "Simulator name or UDID (default: booted)")
	camCmd.AddCommand(camStopCmd)
}
