package cmd

import (
	"fmt"
	"runtime"
)

// DeviceManager defines the interface for platform-specific device operations.
type DeviceManager interface {
	// Name returns the display name of the platform (e.g. "iOS", "Android")
	Name() string

	// List returns all devices for this platform.
	List() ([]Device, error)

	// Start boots the device. noWait applies to Android to skip boot polling.
	Start(deviceID string, noWait bool) (bool, error)

	// Stop shuts down the device.
	Stop(deviceID string) (bool, error)

	// Restart stops and starts the device.
	Restart(deviceID string) (bool, error)

	// Delete permanently removes the device.
	Delete(deviceID string) (bool, error)

	// Erase factory resets the device.
	Erase(deviceID string) (bool, error)

	// Clone duplicates the device (iOS only).
	Clone(sourceDeviceID, newName string) (bool, error)

	// FindRunningDevice finds a running device by its ID.
	// If deviceID is empty, it returns the first/active running device.
	FindRunningDevice(deviceID string) (udid, name string, found bool, err error)
}

// activeManagers holds the registered device managers for the current OS.
var activeManagers []DeviceManager

func init() {
	if runtime.GOOS == DarwinOS {
		activeManagers = append(activeManagers, &IOSManager{})
	}
	activeManagers = append(activeManagers, &AndroidManager{})
}

// GetManagers returns the active device managers.
func GetManagers() []DeviceManager {
	return activeManagers
}

// FindRunningDevice unified search across all active platform managers.
func FindRunningDevice(deviceID string) (udid, name string, isAndroid bool, err error) {
	for _, m := range activeManagers {
		u, n, found, err := m.FindRunningDevice(deviceID)
		if err != nil {
			return "", "", false, err
		}
		if found {
			return u, n, m.Name() == "Android", nil
		}
	}
	if deviceID == "" {
		return "", "", false, ErrNoActiveDevice
	}

	return "", "", false, fmt.Errorf("device %q: %w", deviceID, ErrDeviceNotRunning)
}
