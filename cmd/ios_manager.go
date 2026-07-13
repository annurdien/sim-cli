package cmd

import (
	"fmt"
)

type IOSManager struct{}

func (m *IOSManager) Name() string {
	return "iOS"
}

func (m *IOSManager) List() ([]Device, error) {
	return GetIOSSimulators(), nil
}

func (m *IOSManager) Start(deviceID string, noWait bool) (bool, error) {
	device := FindIOSSimulatorByID(deviceID)
	if device == nil {
		return false, nil
	}

	if err := packageExecutor.Run(CmdXCrun, CmdSimctl, "boot", device.UDID); err != nil {
		return true, fmt.Errorf("failed to boot iOS simulator '%s': %w", deviceID, err)
	}

	if err := packageExecutor.Run("open", "-a", "Simulator"); err != nil {
		PrintInfo(fmt.Sprintf("Warning: could not open Simulator app: %v", err))
	}

	device.State = StateBooted
	if err := SaveLastStartedDevice(device); err != nil {
		PrintInfo(fmt.Sprintf("Warning: could not save last started device: %v", err))
	}

	return true, nil
}

func (m *IOSManager) Stop(deviceID string) (bool, error) {
	device := FindIOSSimulatorByID(deviceID)
	if device == nil {
		return false, nil
	}

	if err := packageExecutor.Run(CmdXCrun, CmdSimctl, "shutdown", device.UDID); err != nil {
		return true, fmt.Errorf("failed to stop iOS simulator '%s': %w", deviceID, err)
	}

	return true, nil
}

func (m *IOSManager) Restart(deviceID string) (bool, error) {
	device := FindIOSSimulatorByID(deviceID)
	if device == nil {
		return false, nil
	}

	_ = packageExecutor.Run(CmdXCrun, CmdSimctl, "shutdown", device.UDID) // Ignore error if already shut down.

	if err := packageExecutor.Run(CmdXCrun, CmdSimctl, "boot", device.UDID); err != nil {
		return true, fmt.Errorf("failed to boot iOS simulator '%s' during restart: %w", deviceID, err)
	}

	if err := packageExecutor.Run("open", "-a", "Simulator"); err != nil {
		PrintInfo(fmt.Sprintf("Warning: could not open Simulator app: %v", err))
	}

	device.State = StateBooted
	if err := SaveLastStartedDevice(device); err != nil {
		PrintInfo(fmt.Sprintf("Warning: could not save last started device: %v", err))
	}

	return true, nil
}

func (m *IOSManager) Delete(deviceID string) (bool, error) {
	device := FindIOSSimulatorByID(deviceID)
	if device == nil {
		return false, nil
	}

	_ = packageExecutor.Run(CmdXCrun, CmdSimctl, "shutdown", device.UDID)

	if err := packageExecutor.Run(CmdXCrun, CmdSimctl, "delete", device.UDID); err != nil {
		return true, fmt.Errorf("failed to delete iOS simulator '%s': %w", deviceID, err)
	}

	return true, nil
}

func (m *IOSManager) Erase(deviceID string) (bool, error) {
	device := FindIOSSimulatorByID(deviceID)
	if device == nil {
		return false, nil
	}

	_ = packageExecutor.Run(CmdXCrun, CmdSimctl, "shutdown", device.UDID)

	if err := packageExecutor.Run(CmdXCrun, CmdSimctl, "erase", device.UDID); err != nil {
		return true, fmt.Errorf("failed to erase iOS simulator '%s': %w", deviceID, err)
	}

	return true, nil
}

func (m *IOSManager) Clone(sourceDeviceID, newName string) (bool, error) {
	device := FindIOSSimulatorByID(sourceDeviceID)
	if device == nil {
		return false, nil
	}

	if err := packageExecutor.Run(CmdXCrun, CmdSimctl, "clone", device.UDID, newName); err != nil {
		return true, fmt.Errorf("failed to clone iOS simulator '%s': %w", sourceDeviceID, err)
	}

	return true, nil
}

func (m *IOSManager) FindRunningDevice(deviceID string) (udid, name string, found bool, err error) {
	if deviceID == "" {
		sims := GetIOSSimulators()
		for _, sim := range sims {
			if sim.State == StateBooted {
				return sim.UDID, sim.Name, true, nil
			}
		}

		return "", "", false, nil // ErrNoRunningIOSSimulator handled in manager.go
	}

	device := FindIOSSimulatorByID(deviceID)
	if device != nil && device.State == StateBooted {
		return device.UDID, device.Name, true, nil
	}

	if device != nil && device.State != StateBooted {
		return "", "", true, fmt.Errorf("device %q: %w", deviceID, ErrDeviceNotRunning)
	}

	return "", "", false, nil
}
