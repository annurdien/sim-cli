package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/spf13/cobra"
)

var pushTemplate bool

var pushCmd = &cobra.Command{
	Use:   "push [device-name-or-udid] <bundle-id> <payload.json>",
	Short: "Send a push notification (iOS only)",
	Long: `Send a simulated push notification to an app on an iOS simulator.
	
Provide a valid APNs payload JSON file. Android emulators do not support this command.`,
	ValidArgsFunction: validDeviceArgs,
	Args:              cobra.RangeArgs(0, 3),
	RunE: func(cmd *cobra.Command, args []string) error {
		if pushTemplate {
			return GeneratePushTemplate()
		}

		if len(args) < 2 {
			return fmt.Errorf("bundle-id and payload.json are required unless using --template") //nolint:err113
		}

		var deviceID, bundleID, payloadPath string
		if len(args) == 2 {
			bundleID = args[0]
			payloadPath = args[1]
		} else {
			deviceID = args[0]
			bundleID = args[1]
			payloadPath = args[2]
		}

		return SendPushNotification(deviceID, bundleID, payloadPath)
	},
}

func init() {
	pushCmd.Flags().BoolVarP(&pushTemplate, "template", "t", false, "Generate a sample push payload template (push.json)")
}

func GeneratePushTemplate() error {
	template := `{
  "aps": {
    "alert": {
      "title": "Test Notification",
      "body": "This is a simulated push notification from sim-cli"
    },
    "sound": "default",
    "badge": 1
  },
  "customKey": "customValue"
}`
	err := os.WriteFile("push.json", []byte(template), 0o644)
	if err != nil {
		return fmt.Errorf("failed to write template: %w", err)
	}

	PrintSuccess("Generated sample push payload at push.json")

	return nil
}

func SendPushNotification(deviceID, bundleID, payloadPath string) error {
	if runtime.GOOS != DarwinOS {
		return ErrIOSMacOnly
	}

	// Validate JSON
	content, err := os.ReadFile(payloadPath)
	if err != nil {
		return fmt.Errorf("failed to read payload file: %w", err)
	}
	if !json.Valid(content) {
		return fmt.Errorf("invalid JSON payload in %s", payloadPath) //nolint:err113
	}

	udid, name, isAndroid, err := FindRunningDevice(deviceID)
	if err != nil {
		return err
	}

	if isAndroid {
		return fmt.Errorf("push notifications are not supported for Android emulators") //nolint:err113
	}

	absPath, err := filepath.Abs(payloadPath)
	if err != nil {
		return fmt.Errorf("invalid payload path: %w", err)
	}

	err = RunSpinner(fmt.Sprintf("Sending push notification to %s on '%s'...", bundleID, name), func() error {
		if pushErr := packageExecutor.Run(CmdXCrun, CmdSimctl, "push", udid, bundleID, absPath); pushErr != nil {
			return fmt.Errorf("failed to send push notification: %w", pushErr)
		}

		return nil
	})

	if err == nil {
		PrintSuccess("Push notification sent successfully.")
	}

	return err
}
