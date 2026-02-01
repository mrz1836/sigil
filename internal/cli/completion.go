package cli

import (
	"os"

	"github.com/spf13/cobra"
)

// completionCmd generates shell completion scripts.
//
//nolint:gochecknoglobals // Cobra CLI pattern requires package-level command variables
var completionCmd = &cobra.Command{
	Use:   "completion [bash|zsh|fish|powershell]",
	Short: "Generate shell completion script",
	Long: `Generate shell completion scripts for sigil.

To load completions:

Bash:
  $ source <(sigil completion bash)

  # To load completions for each session, execute once:
  # Linux:
  $ sigil completion bash > /etc/bash_completion.d/sigil
  # macOS:
  $ sigil completion bash > $(brew --prefix)/etc/bash_completion.d/sigil

Zsh:
  # If shell completion is not already enabled in your environment,
  # you will need to enable it. You can execute the following once:
  $ echo "autoload -U compinit; compinit" >> ~/.zshrc

  # To load completions for each session, execute once:
  $ sigil completion zsh > "${fpath[1]}/_sigil"

  # You will need to start a new shell for this setup to take effect.

Fish:
  $ sigil completion fish | source

  # To load completions for each session, execute once:
  $ sigil completion fish > ~/.config/fish/completions/sigil.fish

PowerShell:
  PS> sigil completion powershell | Out-String | Invoke-Expression

  # To load completions for every new session, run:
  PS> sigil completion powershell > sigil.ps1
  # and source this file from your PowerShell profile.
`,
	DisableFlagsInUseLine: true,
	ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
	Args:                  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
	RunE: func(cmd *cobra.Command, args []string) error {
		switch args[0] {
		case "bash":
			return cmd.Root().GenBashCompletion(os.Stdout)
		case "zsh":
			return cmd.Root().GenZshCompletion(os.Stdout)
		case "fish":
			return cmd.Root().GenFishCompletion(os.Stdout, true)
		case "powershell":
			return cmd.Root().GenPowerShellCompletionWithDesc(os.Stdout)
		}
		return nil
	},
}

//nolint:gochecknoinits // Cobra CLI pattern requires init for command registration
func init() {
	rootCmd.AddCommand(completionCmd)
}
