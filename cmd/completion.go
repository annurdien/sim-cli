package cmd

import (
	"fmt"
	"os"
	"runtime"

	"github.com/spf13/cobra"
)

var completionCmd = &cobra.Command{
	Use:   "completion [bash|zsh|fish|powershell]",
	Short: "Generate completion script",
	Long: `To load completions:

Bash:

  $ source <(sim completion bash)

  # To load completions for each session, execute once:
  # Linux:
  $ sim completion bash > /etc/bash_completion.d/sim
  # macOS:
  $ sim completion bash > /usr/local/etc/bash_completion.d/sim

Zsh:

  # If shell completion is not already enabled in your environment,
  # you will need to enable it.  You can execute the following once:

  $ echo "autoload -U compinit; compinit" >> ~/.zshrc

  # To load completions for each session, execute once:
  $ sim completion zsh > "${fpath[1]}/_sim"

  # You will need to start a new shell for this setup to take effect.

Fish:

  $ sim completion fish | source

  # To load completions for each session, execute once:
  $ sim completion fish > ~/.config/fish/completions/sim.fish

PowerShell:

  PS> sim completion powershell | Out-String | Invoke-Expression

  # To load completions for every new session, run:
  PS> sim completion powershell > sim.ps1
  # and source this file from your PowerShell profile.
`,
	DisableFlagsInUseLine: true,
	ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
	Args:                  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
	Run: func(cmd *cobra.Command, args []string) {
		switch args[0] {
		case "bash":
			if err := cmd.Root().GenBashCompletion(os.Stdout); err != nil {
				PrintError(fmt.Sprintf("Failed to generate bash completion: %v", err))
			}
		case "zsh":
			if err := cmd.Root().GenZshCompletion(os.Stdout); err != nil {
				PrintError(fmt.Sprintf("Failed to generate zsh completion: %v", err))
			}
		case "fish":
			if err := cmd.Root().GenFishCompletion(os.Stdout, true); err != nil {
				PrintError(fmt.Sprintf("Failed to generate fish completion: %v", err))
			}
		case "powershell":
			if err := cmd.Root().GenPowerShellCompletionWithDesc(os.Stdout); err != nil {
				PrintError(fmt.Sprintf("Failed to generate powershell completion: %v", err))
			}
		}
	},
}

// getDeviceNames returns a slice of all available device names for autocompletion.
func getDeviceNames() []string {
	var names []string

	// iOS
	if runtime.GOOS == DarwinOS {
		sims := GetIOSSimulators()
		for _, sim := range sims {
			names = append(names, sim.Name)
			names = append(names, sim.UDID) // Also allow completing by UDID
		}
	}

	// Android
	emulators := GetAndroidEmulators()
	for _, emu := range emulators {
		names = append(names, emu.Name)
	}

	return names
}

// validDeviceArgs is a ValidArgsFunction for commands that take a device name.
func validDeviceArgs(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) != 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	names := getDeviceNames()

	return names, cobra.ShellCompDirectiveNoFileComp
}

// validDeviceAndFileArgs is a ValidArgsFunction for commands that take a device name and an optional file.
func validDeviceAndFileArgs(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) == 0 {
		// First arg: device name
		names := getDeviceNames()
		return names, cobra.ShellCompDirectiveNoFileComp
	}
	// Second arg: file, default completion
	return nil, cobra.ShellCompDirectiveDefault
}
