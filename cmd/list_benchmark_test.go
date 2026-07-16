//nolint:testpackage
package cmd

import (
	"os/exec"
	"testing"
)

type mockBenchmarkExecutor struct {
	iosOutput []byte
}

func (m *mockBenchmarkExecutor) Output(name string, args ...string) ([]byte, error) {
	if name == CmdXCrun {
		return m.iosOutput, nil
	}
	if name == CmdEmulator {
		// mock avds list
		return []byte("Pixel_8_API_34\nPixel_7_API_34\nNexus_5X_API_29\n"), nil
	}
	if name == CmdAdb {
		if len(args) > 0 && args[0] == "devices" {
			return []byte("List of devices attached\nemulator-5554 device\nemulator-5556 offline\n"), nil
		}
		if len(args) > 3 && args[3] == "name" {
			if args[2] == "emulator-5554" {
				return []byte("Pixel_8_API_34\nOK\n"), nil
			}

			return []byte("Unknown_AVD\nOK\n"), nil
		}
	}

	return nil, nil
}

func (m *mockBenchmarkExecutor) Run(name string, args ...string) error { return nil }
func (m *mockBenchmarkExecutor) Start(name string, args ...string) (*exec.Cmd, error) {
	return nil, nil
}

var benchmarkIOSOutput = []byte(`{
  "devices" : {
    "com.apple.CoreSimulator.SimRuntime.iOS-17-0" : [
      {
        "udid" : "1234-5678-9012-3456",
        "isAvailable" : true,
        "deviceTypeIdentifier" : "com.apple.CoreSimulator.SimDeviceType.iPhone-15",
        "state" : "Booted",
        "name" : "iPhone 15"
      },
      {
        "udid" : "2345-6789-0123-4567",
        "isAvailable" : true,
        "deviceTypeIdentifier" : "com.apple.CoreSimulator.SimDeviceType.iPhone-15-Pro",
        "state" : "Shutdown",
        "name" : "iPhone 15 Pro"
      }
    ],
    "com.apple.CoreSimulator.SimRuntime.iOS-16-4" : [
      {
        "udid" : "3456-7890-1234-5678",
        "isAvailable" : true,
        "deviceTypeIdentifier" : "com.apple.CoreSimulator.SimDeviceType.iPhone-14",
        "state" : "Shutdown",
        "name" : "iPhone 14"
      }
    ]
  }
}`)

func BenchmarkGetIOSSimulators(b *testing.B) {
	mockExec := &mockBenchmarkExecutor{
		iosOutput: benchmarkIOSOutput,
	}
	SetExecutor(mockExec)
	defer SetExecutor(&OSCommandExecutor{})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = GetIOSSimulators()
	}
}

func BenchmarkGetAndroidEmulators(b *testing.B) {
	mockExec := &mockBenchmarkExecutor{}
	SetExecutor(mockExec)
	defer SetExecutor(&OSCommandExecutor{})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = GetAndroidEmulators()
	}
}

func BenchmarkFormatRuntime(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = FormatRuntime("com.apple.CoreSimulator.SimRuntime.iOS-17-0")
	}
}
