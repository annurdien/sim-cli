package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
)

var (
	createIOS      bool
	createAndroid  bool
	createName     string
	createType     string
	createRuntime  string
	createListType bool
)

var createCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new simulator or emulator",
	RunE: func(cmd *cobra.Command, args []string) error {
		if !createIOS && !createAndroid && !createListType {
			if createName == "" && createType == "" && createRuntime == "" {
				if err := runCreateWizard(); err != nil {
					return err
				}
			} else {
				return fmt.Errorf("must specify either --ios or --android") //nolint:err113
			}
		}

		if createIOS && createAndroid {
			return fmt.Errorf("cannot specify both --ios and --android") //nolint:err113
		}

		if createListType {
			if createIOS {
				return ListIOSCreateTypes()
			}

			return ListAndroidCreateTypes()
		}

		if createName == "" || createType == "" || createRuntime == "" {
			return fmt.Errorf("name, type, and runtime are required to create a device") //nolint:err113
		}

		if createIOS {
			err := RunSpinner(fmt.Sprintf("Creating iOS Simulator %q...", createName), func() error {
				return CreateIOSDevice(createName, createType, createRuntime)
			})
			if err == nil {
				PrintSuccess(fmt.Sprintf("Successfully created iOS Simulator %q", createName))
			}

			return err
		}

		err := RunSpinner(fmt.Sprintf("Creating Android Emulator %q...", createName), func() error {
			return CreateAndroidDevice(createName, createType, createRuntime)
		})
		if err == nil {
			PrintSuccess(fmt.Sprintf("Successfully created Android Emulator %q", createName))
		}

		return err
	},
}

func init() {
	createCmd.Flags().BoolVar(&createIOS, "ios", false, "Target iOS simulator")
	createCmd.Flags().BoolVar(&createAndroid, "android", false, "Target Android emulator")
	createCmd.Flags().StringVarP(&createName, "name", "n", "", "Name of the new device")
	createCmd.Flags().StringVarP(&createType, "type", "t", "", "Device type identifier")
	createCmd.Flags().StringVarP(&createRuntime, "runtime", "r", "", "Runtime/OS identifier")
	createCmd.Flags().BoolVar(&createListType, "list-types", false, "List available device types and runtimes")
}

func ListIOSCreateTypes() error {
	if runtime.GOOS != DarwinOS {
		return ErrIOSMacOnly
	}

	PrintInfo("--- iOS Device Types ---")

	out, err := packageExecutor.Output(CmdXCrun, CmdSimctl, "list", "devicetypes", "--json")
	if err == nil {
		var list struct {
			DeviceTypes []struct {
				Name       string `json:"name"`
				Identifier string `json:"identifier"`
			} `json:"devicetypes"`
		}

		if json.Unmarshal(out, &list) == nil {
			var rows [][]string
			for _, dt := range list.DeviceTypes {
				rows = append(rows, []string{dt.Name, dt.Identifier})
			}
			RenderTable([]string{"iOS Device Type", "Identifier"}, rows)
		}
	}

	outRun, errRun := packageExecutor.Output(CmdXCrun, CmdSimctl, "list", "runtimes", "--json")
	if errRun == nil {
		var runList struct {
			Runtimes []struct {
				Name        string `json:"name"`
				Identifier  string `json:"identifier"`
				IsAvailable bool   `json:"isAvailable"`
			} `json:"runtimes"`
		}

		if json.Unmarshal(outRun, &runList) == nil {
			var rows [][]string
			for _, r := range runList.Runtimes {
				if r.IsAvailable {
					rows = append(rows, []string{r.Name, r.Identifier})
				}
			}
			RenderTable([]string{"iOS Runtime", "Identifier"}, rows)
		}
	}

	return nil
}

func CreateIOSDevice(name, deviceType, runtimeID string) error {
	if runtime.GOOS != DarwinOS {
		return ErrIOSMacOnly
	}

	if err := packageExecutor.Run(CmdXCrun, CmdSimctl, "create", name, deviceType, runtimeID); err != nil {
		return fmt.Errorf("failed to create iOS simulator: %w", err)
	}

	return nil
}

func ListAndroidCreateTypes() error {
	out, err := packageExecutor.Output("sdkmanager", "--list")
	if err != nil {
		PrintInfo("Warning: sdkmanager not found or failed.")
	} else {
		var rows [][]string
		lines := strings.Split(string(out), "\n")
		for _, line := range lines {
			if strings.Contains(line, "system-images;") {
				parts := strings.SplitN(line, " ", 2)
				if len(parts) > 0 {
					rows = append(rows, []string{parts[0]})
				}
			}
		}
		if len(rows) > 0 {
			RenderTable([]string{"Android System Images"}, rows)
		}
	}

	outAvd, errAvd := packageExecutor.Output("avdmanager", "list", "device")
	if errAvd != nil {
		PrintInfo("Warning: avdmanager not found or failed.")
	} else {
		var rows [][]string
		lines := strings.Split(string(outAvd), "\n")
		for _, line := range lines {
			if strings.HasPrefix(strings.TrimSpace(line), "id:") {
				rows = append(rows, []string{strings.TrimSpace(line)})
			}
		}
		if len(rows) > 0 {
			RenderTable([]string{"Android Device Types"}, rows)
		}
	}

	return nil
}

func CreateAndroidDevice(name, deviceType, runtimeID string) error {
	args := []string{"create", "avd", "-n", name, "-k", runtimeID}
	if deviceType != "" && deviceType != "default" {
		args = append(args, "--device", deviceType)
	}

	// avdmanager prompts for custom hardware profile. We pipe "no\n" to it.
	cmd := exec.Command(CmdAvdManager, args...)
	cmd.Stdin = strings.NewReader("no\n")

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create Android emulator: %w", err)
	}

	return nil
}

func fetchIOSDeviceTypes() ([]string, error) {
	out, err := packageExecutor.Output(CmdXCrun, CmdSimctl, "list", "devicetypes", "--json")
	if err != nil {
		return nil, err
	}
	var list struct {
		DeviceTypes []struct {
			Identifier string `json:"identifier"`
		} `json:"devicetypes"`
	}
	if err := json.Unmarshal(out, &list); err != nil {
		return nil, err
	}
	var res []string
	for _, dt := range list.DeviceTypes {
		res = append(res, dt.Identifier)
	}

	return res, nil
}

func fetchIOSRuntimes() ([]string, error) {
	out, err := packageExecutor.Output(CmdXCrun, CmdSimctl, "list", "runtimes", "--json")
	if err != nil {
		return nil, err
	}
	var runList struct {
		Runtimes []struct {
			Identifier  string `json:"identifier"`
			IsAvailable bool   `json:"isAvailable"`
		} `json:"runtimes"`
	}
	if err := json.Unmarshal(out, &runList); err != nil {
		return nil, err
	}
	var res []string
	for _, r := range runList.Runtimes {
		if r.IsAvailable {
			res = append(res, r.Identifier)
		}
	}

	return res, nil
}

func fetchAndroidSystemImages() ([]string, error) {
	out, err := packageExecutor.Output("sdkmanager", "--list")
	if err != nil {
		return nil, err
	}
	var res []string
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		if strings.Contains(line, "system-images;") {
			parts := strings.SplitN(line, " ", 2)
			if len(parts) > 0 {
				res = append(res, parts[0])
			}
		}
	}

	return res, nil
}

func fetchAndroidDeviceTypes() ([]string, error) {
	out, err := packageExecutor.Output("avdmanager", "list", "device")
	if err != nil {
		return nil, err
	}
	var res []string
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "id:") {
			parts := strings.SplitN(strings.TrimSpace(line), " ", 2)
			if len(parts) > 1 {
				res = append(res, strings.TrimSpace(parts[1]))
			} else {
				res = append(res, strings.TrimPrefix(strings.TrimSpace(line), "id: "))
			}
		}
	}

	return res, nil
}

func runCreateWizard() error {
	var platform string
	err := huh.NewSelect[string]().
		Title("Select Platform").
		Options(
			huh.NewOption("iOS Simulator", PlatformIOS),
			huh.NewOption("Android Emulator", "android"),
		).
		Value(&platform).
		Run()
	if err != nil {
		return err
	}

	var runtimes []string
	var deviceTypes []string

	err = RunSpinner("Fetching available runtimes and types...", func() error {
		if platform == PlatformIOS {
			var err1, err2 error
			runtimes, err1 = fetchIOSRuntimes()
			if err1 != nil {
				return err1
			}
			deviceTypes, err2 = fetchIOSDeviceTypes()
			if err2 != nil {
				return err2
			}
		} else {
			var err1, err2 error
			runtimes, err1 = fetchAndroidSystemImages()
			if err1 != nil {
				return err1
			}
			deviceTypes, err2 = fetchAndroidDeviceTypes()
			if err2 != nil {
				return err2
			}
		}

		return nil
	})
	if err != nil {
		return err
	}

	var runtimeID string
	var typeID string
	var deviceName string

	var runtimeOptions []huh.Option[string]
	for _, r := range runtimes {
		runtimeOptions = append(runtimeOptions, huh.NewOption(r, r))
	}
	var typeOptions []huh.Option[string]
	for _, t := range deviceTypes {
		typeOptions = append(typeOptions, huh.NewOption(t, t))
	}

	if len(runtimeOptions) == 0 {
		return fmt.Errorf("no runtimes found for %s", platform) //nolint:err113
	}
	if len(typeOptions) == 0 {
		typeOptions = append(typeOptions, huh.NewOption("default", "default"))
	}

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Select Runtime").
				Options(runtimeOptions...).
				Value(&runtimeID).
				Height(8),
			huh.NewSelect[string]().
				Title("Select Device Type").
				Options(typeOptions...).
				Value(&typeID).
				Height(8),
			huh.NewInput().
				Title("Device Name").
				Value(&deviceName).
				Validate(func(s string) error {
					if strings.TrimSpace(s) == "" {
						return errors.New("name cannot be empty") //nolint:err113
					}

					return nil
				}),
		),
	)

	if err := form.Run(); err != nil {
		return err
	}

	if platform == PlatformIOS {
		createIOS = true
	} else {
		createAndroid = true
	}
	createName = deviceName
	createType = typeID
	createRuntime = runtimeID

	return nil
}
