package cmd

import (
	"fmt"
	"strings"
	"time"
)

// androidBootTimeout is the maximum time to wait for an Android emulator to finish booting.
const androidBootTimeout = 120 * time.Second

// androidBootPollInterval is how often to poll boot status.
const androidBootPollInterval = 3 * time.Second

type AndroidManager struct{}

func (m *AndroidManager) Name() string {
	return NameAndroid
}

func (m *AndroidManager) List() ([]Device, error) {
	return GetAndroidEmulators(), nil
}

func (m *AndroidManager) Start(deviceID string, noWait bool) (bool, error) {
	if IsAndroidEmulatorRunning(deviceID) {
		PrintInfo(fmt.Sprintf("Android emulator '%s' is already running", deviceID))

		udid, name := FindRunningAndroidEmulator(deviceID)
		device := &Device{
			Name:  name,
			UDID:  udid,
			Type:  TypeAndroidEmulator,
			State: StateBooted,
		}
		if err := SaveLastStartedDevice(device); err != nil {
			PrintInfo(fmt.Sprintf("Warning: could not save last started device: %v", err))
		}

		return true, nil
	}

	if !DoesAndroidAVDExist(deviceID) {
		return false, nil
	}

	_, err := packageExecutor.Start(CmdEmulator, "-avd", deviceID)
	if err != nil {
		return true, fmt.Errorf("failed to start Android emulator '%s': %w", deviceID, err)
	}

	if noWait {
		device := &Device{
			Name:  deviceID,
			UDID:  "starting",
			Type:  TypeAndroidEmulator,
			State: StateBooted,
		}
		if err := SaveLastStartedDevice(device); err != nil {
			PrintInfo(fmt.Sprintf("Warning: could not save last started device: %v", err))
		}

		return true, nil
	}

	udid, bootErr := waitForAndroidBoot(deviceID)
	if bootErr != nil {
		PrintInfo(fmt.Sprintf("Warning: emulator may still be booting: %v", bootErr))
		PrintInfo("Use 'sim last' to check the saved device once it finishes booting.")
		udid = "starting"
	}

	device := &Device{
		Name:  deviceID,
		UDID:  udid,
		Type:  TypeAndroidEmulator,
		State: StateBooted,
	}
	if err := SaveLastStartedDevice(device); err != nil {
		PrintInfo(fmt.Sprintf("Warning: could not save last started device: %v", err))
	}

	return true, nil
}

// waitForAndroidBoot polls adb until the emulator with the given AVD name has
// fully booted (sys.boot_completed == 1). Returns the emulator serial (UDID).
func waitForAndroidBoot(avdName string) (string, error) {
	deadline := time.Now().Add(androidBootTimeout)

	for time.Now().Before(deadline) {
		udid, _ := FindRunningAndroidEmulator(avdName)
		if udid != "" {
			// Check if the system has fully booted.
			out, err := packageExecutor.Output(CmdAdb, "-s", udid, "shell", "getprop", "sys.boot_completed")
			if err == nil && strings.TrimSpace(string(out)) == "1" {
				return udid, nil
			}
		}

		time.Sleep(androidBootPollInterval)
	}

	// Last attempt: maybe it booted right at the deadline.
	if udid, _ := FindRunningAndroidEmulator(avdName); udid != "" {
		return udid, nil
	}

	return "starting", fmt.Errorf("timed out waiting for Android emulator '%s' to boot after %s", avdName, androidBootTimeout) //nolint:err113
}

func (m *AndroidManager) Stop(deviceID string) (bool, error) {
	udid, _ := FindRunningAndroidEmulator(deviceID)
	if udid == "" {
		return false, nil
	}

	if err := packageExecutor.Run(CmdAdb, "-s", udid, "emu", "kill"); err != nil {
		return true, fmt.Errorf("failed to stop Android emulator '%s': %w", deviceID, err)
	}

	return true, nil
}

func (m *AndroidManager) Restart(deviceID string) (bool, error) {
	udid, _ := FindRunningAndroidEmulator(deviceID)
	if udid == "" {
		return false, nil
	}

	if _, err := m.Stop(deviceID); err != nil {
		PrintInfo(fmt.Sprintf("Warning: failed to stop device before restart: %v", err))
	}

	return m.Start(deviceID, false)
}

func (m *AndroidManager) Delete(deviceID string) (bool, error) {
	if !DoesAndroidAVDExist(deviceID) {
		return false, nil
	}

	if _, err := m.Stop(deviceID); err != nil {
		PrintInfo(fmt.Sprintf("Warning: failed to stop device before delete: %v", err))
	}

	if err := packageExecutor.Run(CmdAvdManager, "delete", "avd", "-n", deviceID); err != nil {
		return true, fmt.Errorf("failed to delete Android emulator '%s': %w", deviceID, err)
	}

	return true, nil
}

func (m *AndroidManager) Erase(deviceID string) (bool, error) {
	if !DoesAndroidAVDExist(deviceID) {
		return false, nil
	}

	if _, err := m.Stop(deviceID); err != nil {
		PrintInfo(fmt.Sprintf("Warning: failed to stop device before erase: %v", err))
	}

	_, err := packageExecutor.Start(CmdEmulator, "-avd", deviceID, "-wipe-data")
	if err != nil {
		return true, fmt.Errorf("failed to erase Android emulator '%s': %w", deviceID, err)
	}

	return true, nil
}

func (m *AndroidManager) Clone(sourceDeviceID, newName string) (bool, error) {
	if DoesAndroidAVDExist(sourceDeviceID) {
		return true, ErrAndroidCloneNotSupported
	}

	return false, nil
}

func (m *AndroidManager) FindRunningDevice(deviceID string) (udid, name string, found bool, err error) {
	if deviceID == "" {
		u, n := FindRunningAndroidEmulator("")
		if u != "" {
			return u, n, true, nil
		}

		return "", "", false, nil
	}

	u, n := FindRunningAndroidEmulator(deviceID)
	if u != "" {
		return u, n, true, nil
	}

	if DoesAndroidAVDExist(deviceID) {
		return "", "", true, fmt.Errorf("device %q: %w", deviceID, ErrDeviceNotRunning)
	}

	return "", "", false, nil
}

// DoesAndroidAVDExist checks whether an AVD with the given name is defined.
func DoesAndroidAVDExist(avdName string) bool {
	output, err := packageExecutor.Output(CmdEmulator, "-list-avds")
	if err != nil {
		return false
	}

	lines := strings.SplitSeq(strings.TrimSpace(string(output)), "\n")
	for line := range lines {
		if strings.TrimSpace(line) == avdName {
			return true
		}
	}

	return false
}

// IsAndroidEmulatorRunning reports whether an emulator with the given AVD name is currently running.
func IsAndroidEmulatorRunning(avdName string) bool {
	udid, _ := FindRunningAndroidEmulator(avdName)

	return udid != ""
}
