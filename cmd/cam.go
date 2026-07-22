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
	"sync"
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

func shmPath(udid string) string     { return fmt.Sprintf("/tmp/minisimcam.%s.frames", udid) }
func pidFilePath(udid string) string { return fmt.Sprintf("/tmp/minisimcam.%s.pid", udid) }
func statusFilePath(udid string) string {
	primary := fmt.Sprintf("/tmp/minisimcam.%s.status", udid)
	if _, err := os.Stat(primary); err == nil {
		return primary
	}
	secondary := filepath.Join(os.TempDir(), fmt.Sprintf("minisimcam.%s.status", udid))
	if _, err := os.Stat(secondary); err == nil {
		return secondary
	}
	return primary
}

// resolveEmbeddedBinDir returns the directory containing extracted embedded
// binaries, or "" if the embedded assets are stubs (dev builds without cam).
// The result is cached after the first successful extraction.
var (
	embeddedBinDirOnce  sync.Once
	embeddedBinDirValue string
)

func resolveEmbeddedBinDir() string {
	embeddedBinDirOnce.Do(func() {
		if binDir, err := ensureExtractedAssets(); err == nil {
			embeddedBinDirValue = binDir
		}
	})
	return embeddedBinDirValue
}

func frameHostBin(mscDir string) string {
	if binDir := resolveEmbeddedBinDir(); binDir != "" {
		extracted := filepath.Join(binDir, "FrameHost")
		if _, err := os.Stat(extracted); err == nil {
			return extracted
		}
	}
	return filepath.Join(mscDir, ".build", "release", "FrameHost")
}

func injectorDylib(mscDir string) string {
	if binDir := resolveEmbeddedBinDir(); binDir != "" {
		extracted := filepath.Join(binDir, "MiniCamInject.dylib")
		if _, err := os.Stat(extracted); err == nil {
			return extracted
		}
	}
	return filepath.Join(mscDir, ".build", "injector", "MiniCamInject.dylib")
}

func findRunningIOSSimulator(deviceID string) (udid, name string, err error) {
	udid, name, isAndroid, err := FindRunningDevice(deviceID)
	if err != nil || udid == "" {
		return "", "", err
	}
	if isAndroid {
		return "", "", fmt.Errorf("MiniSimCam only supports booted iOS Simulators, not Android emulators")
	}
	return udid, name, nil
}

// waitForFrameHostReady rejects immediate FrameHost failures (for example an
// invalid camera ID) instead of printing a misleading successful start.
func waitForFrameHostReady(c *exec.Cmd, statusPath string) error {
	exited := make(chan error, 1)
	go func() { exited <- c.Wait() }()

	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case err := <-exited:
			if err == nil {
				return fmt.Errorf("FrameHost exited during startup")
			}
			return fmt.Errorf("FrameHost failed during startup: %w", err)
		case <-ticker.C:
			data, err := os.ReadFile(statusPath)
			if err != nil {
				continue
			}
			var status camFrameLoopStatus
			if json.Unmarshal(data, &status) == nil && status.HostPID == int32(c.Process.Pid) {
				return nil
			}
		case <-deadline.C:
			_ = c.Process.Kill()
			return fmt.Errorf("FrameHost did not become ready within 5 seconds")
		}
	}
}

func frameHostFPS(udid string) int {
	data, err := os.ReadFile(statusFilePath(udid))
	if err != nil {
		return DefaultCamFPS
	}
	var status camFrameLoopStatus
	if err := json.Unmarshal(data, &status); err != nil || status.FPS < 1 || status.FPS > 120 {
		return DefaultCamFPS
	}
	return status.FPS
}

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
  sim cam
  sim cam start --image test.png
  sim cam launch --bundle-id com.example.MyApp
  sim cam status
  sim cam stop`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if runtime.GOOS != DarwinOS {
			return fmt.Errorf("cam is only supported on macOS")
		}
		return runCamDashboard()
	},
}

// ---------------------------------------------------------------------------
// sim cam start
// ---------------------------------------------------------------------------

var (
	camStartImage     string
	camStartBars      bool
	camStartCamera    bool
	camStartCameraID  string
	camStartScaleMode string
	camStartWidth     int
	camStartHeight    int
	camStartFPS       int
	camStartDevice    string
)

var camStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the FrameHost (frame producer)",
	Long: `Start the FrameHost macOS process that writes BGRA frames to shared memory.

Examples:
  sim cam start --image ./test-card.png
  sim cam start --bars --width 1920 --height 1080 --fps 60
  sim cam start --camera
  sim cam start --camera --camera-id "iPhone"
  sim cam start --image qr.png --device <UDID>`,

	RunE: func(cmd *cobra.Command, args []string) error {
		if runtime.GOOS != DarwinOS {
			return fmt.Errorf("cam start is only supported on macOS")
		}
		sourceCount := 0
		if camStartImage != "" {
			sourceCount++
		}
		if camStartBars {
			sourceCount++
		}
		if camStartCamera {
			sourceCount++
		}
		if sourceCount != 1 {
			return fmt.Errorf("provide exactly one of --image <path>, --bars, or --camera")
		}
		if camStartWidth <= 0 || camStartHeight <= 0 || camStartWidth > 3840 || camStartHeight > 3840 {
			return fmt.Errorf("--width and --height must be between 1 and 3840")
		}
		if camStartFPS < 1 || camStartFPS > 120 {
			return fmt.Errorf("--fps must be between 1 and 120")
		}
		if camStartCameraID != "" && !camStartCamera {
			return fmt.Errorf("--camera-id requires --camera")
		}
		if cmd.Flags().Changed("scale-mode") && !camStartCamera {
			return fmt.Errorf("--scale-mode requires --camera")
		}

		// Resolve UDID.
		udid, _, err := findRunningIOSSimulator(camStartDevice)
		if err != nil || udid == "" {
			return fmt.Errorf("no booted iOS simulator found — boot one first (%w)", err)
		}

		mscDir := miniSimCamDir()
		bin := frameHostBin(mscDir)
		if _, err := os.Stat(bin); err != nil {
			return fmt.Errorf("FrameHost not found — run 'sim cam build' first (or use a release build with embedded binaries)")
		}

		// Kill any existing FrameHost for this UDID.
		if err := stopFrameHost(udid); err != nil {
			return err
		}
		_ = os.Remove(statusFilePath(udid))

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
			if camStartCameraID != "" {
				hostArgs = append(hostArgs, "--camera-id", camStartCameraID)
			}
			if camStartScaleMode != "" {
				hostArgs = append(hostArgs, "--scale-mode", camStartScaleMode)
			}
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

		if err := waitForFrameHostReady(c, statusFilePath(udid)); err != nil {
			return err
		}

		source := camStartImage
		if camStartBars {
			source = "color-bars"
		} else if camStartCamera {
			if camStartCameraID != "" {
				source = camStartCameraID
			} else {
				source = "mac-camera (default)"
			}
		}
		PrintSuccess(fmt.Sprintf(
			"FrameHost started — source=%s %dx%d @ %d fps (simulator %s)",
			source, camStartWidth, camStartHeight, camStartFPS, udid,
		))
		return nil
	},
}

// ---------------------------------------------------------------------------
// sim cam list
// ---------------------------------------------------------------------------

var camListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all available cameras on this Mac",
	Long: `Enumerate video capture devices available to FrameHost.
Includes built-in FaceTime camera, Continuity Camera (iPhone/iPad), and external USB cameras.

Requires FrameHost to be built first (sim cam build).

Example:
  sim cam list`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if runtime.GOOS != DarwinOS {
			return fmt.Errorf("cam list is only supported on macOS")
		}
		mscDir := miniSimCamDir()
		bin := frameHostBin(mscDir)
		if _, err := os.Stat(bin); err != nil {
			return fmt.Errorf("FrameHost not found — run 'sim cam build' first (or use a release build with embedded binaries)")
		}
		c := exec.Command(bin, "--list-cameras", "--udid", "00000000-0000-0000-0000-000000000000")
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr
		return c.Run()
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

		udid, _, err := findRunningIOSSimulator(camLaunchDevice)
		if err != nil || udid == "" {
			return fmt.Errorf("no booted iOS simulator found (%w)", err)
		}

		mscDir := miniSimCamDir()
		dylib := injectorDylib(mscDir)
		if _, err := os.Stat(dylib); err != nil {
			return fmt.Errorf("MiniCamInject.dylib not found — run 'sim cam build' first (or use a release build with embedded binaries)")
		}

		shm := shmPath(udid)
		fps := frameHostFPS(udid)

		PrintInfo(fmt.Sprintf("Launching %s on %s", camLaunchBundle, udid))
		PrintInfo(fmt.Sprintf("  dylib: %s", dylib))
		PrintInfo(fmt.Sprintf("  shm:   %s", shm))
		PrintInfo(fmt.Sprintf("  fps:   %d", fps))

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
				if writeErr := os.WriteFile(entPath, []byte(entXML), 0o644); writeErr != nil {
					PrintInfo(fmt.Sprintf("  Warning: cannot write entitlements file: %v", writeErr))
				} else if signErr := exec.Command("codesign", "-f", "-s", "-", "--entitlements", entPath, appPath).Run(); signErr != nil {
					PrintInfo(fmt.Sprintf("  Warning: codesign re-sign failed: %v", signErr))
				} else {
					PrintInfo("  Re-signed with get-task-allow.")
				}
			}
		}

		c := exec.Command("xcrun", "simctl", "launch", udid, camLaunchBundle)
		c.Env = append(
			os.Environ(),
			"SIMCTL_CHILD_DYLD_INSERT_LIBRARIES="+dylib,
			"SIMCTL_CHILD_MINISIMCAM_PATH="+shm,
			"SIMCTL_CHILD_MINISIMCAM_FPS="+strconv.Itoa(fps),
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

// camFrameLoopStatus mirrors FrameLoopStatus defined in FrameLoop.swift and
// the richer status written by CameraSource.
type camFrameLoopStatus struct {
	UDID               string  `json:"udid"`
	Source             string  `json:"source"`
	CameraName         string  `json:"cameraName,omitempty"`
	CameraType         string  `json:"cameraType,omitempty"`
	Width              int     `json:"width"`
	Height             int     `json:"height"`
	FPS                int     `json:"fps"`
	FramesProduced     uint64  `json:"framesProduced"`
	HostPID            int32   `json:"hostPID"`
	StartedAt          string  `json:"startedAt"`
	LastFrameAgeMs     float64 `json:"lastFrameAgeMs"`
	LastDisconnectedAt string  `json:"lastDisconnectedAt,omitempty"`
	Running            bool    `json:"running"`
}

var camStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show FrameHost status and frame statistics",
	RunE: func(cmd *cobra.Command, args []string) error {
		if runtime.GOOS != DarwinOS {
			return fmt.Errorf("cam status is only supported on macOS")
		}

		udid, name, err := findRunningIOSSimulator(camStatusDevice)
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
		}
		if st.CameraName != "" {
			lines = append(lines, fmt.Sprintf("Camera:          %s", st.CameraName))
		}
		if st.CameraType != "" {
			lines = append(lines, fmt.Sprintf("Camera type:     %s", st.CameraType))
		}
		if st.Source == "disconnected" && st.LastDisconnectedAt != "" {
			lines = append(lines, fmt.Sprintf("⚠️  Disconnected at: %s", st.LastDisconnectedAt))
		}
		lines = append(
			lines,
			fmt.Sprintf("Resolution:      %dx%d BGRA", st.Width, st.Height),
			fmt.Sprintf("Frame rate:      %d fps", st.FPS),
			fmt.Sprintf("Frames produced: %d", st.FramesProduced),
			fmt.Sprintf("Last frame age:  %.0f ms", st.LastFrameAgeMs),
			fmt.Sprintf("Host PID:        %d", st.HostPID),
			fmt.Sprintf("Started at:      %s", st.StartedAt),
			fmt.Sprintf("Running:         %v", st.Running),
		)

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

		udid, _, err := findRunningIOSSimulator(camStopDevice)
		if err != nil || udid == "" {
			if camStopDevice == "" {
				_ = stopFrameHost("")
				PrintSuccess("All FrameHost processes stopped.")
				return nil
			}
			_ = stopFrameHost(camStopDevice)
			PrintSuccess("FrameHost stopped.")
			return nil
		}

		if err := stopFrameHost(udid); err != nil {
			return err
		}
		PrintSuccess("FrameHost stopped.")
		return nil
	},
}

func setGlobalSimEnv(udid string, fps int) {
	if udid == "" {
		return
	}
	mscDir := miniSimCamDir()
	dylib := injectorDylib(mscDir)
	shm := shmPath(udid)

	_ = exec.Command("xcrun", "simctl", "spawn", udid, "launchctl", "setenv", "DYLD_INSERT_LIBRARIES", dylib).Run()
	_ = exec.Command("xcrun", "simctl", "spawn", udid, "launchctl", "setenv", "MINISIMCAM_PATH", shm).Run()
	_ = exec.Command("xcrun", "simctl", "spawn", udid, "launchctl", "setenv", "MINISIMCAM_FPS", strconv.Itoa(fps)).Run()
}

func unsetGlobalSimEnv(udid string) {
	if udid == "" {
		return
	}
	_ = exec.Command("xcrun", "simctl", "spawn", udid, "launchctl", "unsetenv", "DYLD_INSERT_LIBRARIES").Run()
	_ = exec.Command("xcrun", "simctl", "spawn", udid, "launchctl", "unsetenv", "MINISIMCAM_PATH").Run()
	_ = exec.Command("xcrun", "simctl", "spawn", udid, "launchctl", "unsetenv", "MINISIMCAM_FPS").Run()
}

// stopFrameHost scans all running processes and terminates ALL FrameHost processes
// associated with the target UDID immediately.
func stopFrameHost(udid string) error {
	pidPath := pidFilePath(udid)
	statusPath := statusFilePath(udid)

	unsetGlobalSimEnv(udid)

	defer func() {
		if udid != "" {
			_ = os.Remove(pidPath)
			_ = os.Remove(statusPath)
			_ = os.Remove(fmt.Sprintf("/tmp/minisimcam.%s.pid", udid))
			_ = os.Remove(fmt.Sprintf("/tmp/minisimcam.%s.status", udid))
		} else {
			if files, err := filepath.Glob("/tmp/minisimcam.*.pid"); err == nil {
				for _, f := range files {
					_ = os.Remove(f)
				}
			}
			if files, err := filepath.Glob("/tmp/minisimcam.*.status"); err == nil {
				for _, f := range files {
					_ = os.Remove(f)
				}
			}
			if files, err := filepath.Glob(filepath.Join(os.TempDir(), "minisimcam.*.status")); err == nil {
				for _, f := range files {
					_ = os.Remove(f)
				}
			}
		}
	}()

	out, err := exec.Command("ps", "-eo", "pid,command").Output()
	if err != nil {
		return nil
	}

	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "FrameHost") && (udid == "" || strings.Contains(line, udid)) {
			fields := strings.Fields(line)
			if len(fields) > 0 {
				if pid, err := strconv.Atoi(fields[0]); err == nil {
					if proc, err := os.FindProcess(pid); err == nil {
						_ = proc.Kill()
					}
				}
			}
		}
	}

	return nil
}

// ---------------------------------------------------------------------------
// init: register sub-commands and flags
// ---------------------------------------------------------------------------

func init() {
	// cam-level flag: --msc-dir override (hidden for end users)
	camCmd.PersistentFlags().StringVar(&camMscDir, "msc-dir", "", "Path to MiniSimCam directory (default: auto-detected)")
	_ = camCmd.PersistentFlags().MarkHidden("msc-dir")

	// start
	camStartCmd.Flags().StringVar(&camStartImage, "image", "", "Path to PNG or JPEG source image")
	camStartCmd.Flags().BoolVar(&camStartBars, "bars", false, "Use synthetic SMPTE color-bar image")
	camStartCmd.Flags().BoolVar(&camStartCamera, "camera", false, "Use the Mac's physical camera as a live source")
	camStartCmd.Flags().StringVar(&camStartCameraID, "camera-id", "", "Camera name (substring) or uniqueID to select (requires --camera)")
	camStartCmd.Flags().StringVar(&camStartScaleMode, "scale-mode", "", "How to scale: 'fill' (crop, fast) or 'fit' (letterbox, no crop). Requires --camera. (default: fill)")
	camStartCmd.Flags().IntVar(&camStartWidth, "width", DefaultCamWidth, "Frame width in pixels")
	camStartCmd.Flags().IntVar(&camStartHeight, "height", DefaultCamHeight, "Frame height in pixels")
	camStartCmd.Flags().IntVar(&camStartFPS, "fps", DefaultCamFPS, "Frames per second (1-120)")
	camStartCmd.Flags().StringVar(&camStartDevice, "device", "", "Simulator name or UDID (default: booted)")
	camCmd.AddCommand(camStartCmd)

	// list
	camCmd.AddCommand(camListCmd)

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
