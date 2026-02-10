package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAllCommandsHaveShortDescription walks the entire command tree and
// verifies that every command has a non-empty Short description.
func TestAllCommandsHaveShortDescription(t *testing.T) {
	walkCommands(rootCmd, func(cmd *cobra.Command) {
		t.Run(cmd.CommandPath(), func(t *testing.T) {
			assert.NotEmpty(t, cmd.Short,
				"%s: missing Short description", cmd.CommandPath())
		})
	})
}

// TestAllCommandsHaveLongDescription walks the entire command tree and
// verifies that every command has a non-empty Long description.
func TestAllCommandsHaveLongDescription(t *testing.T) {
	walkCommands(rootCmd, func(cmd *cobra.Command) {
		t.Run(cmd.CommandPath(), func(t *testing.T) {
			assert.NotEmpty(t, cmd.Long,
				"%s: missing Long description", cmd.CommandPath())
		})
	})
}

// TestLeafCommandsHaveExamples verifies that every leaf command (one
// with RunE or Run) has a non-empty Example field.
func TestLeafCommandsHaveExamples(t *testing.T) {
	walkCommands(rootCmd, func(cmd *cobra.Command) {
		if cmd.RunE == nil && cmd.Run == nil {
			return // parent/group command — not required to have examples
		}
		t.Run(cmd.CommandPath(), func(t *testing.T) {
			assert.NotEmpty(t, cmd.Example,
				"%s: leaf command missing Example field", cmd.CommandPath())
		})
	})
}

// TestNoEmbeddedExamplesInLong ensures no command embeds "Example:" or
// "Examples:" text inside the Long field. Examples should use the
// dedicated Example field so Cobra renders them in a separate section.
func TestNoEmbeddedExamplesInLong(t *testing.T) {
	walkCommands(rootCmd, func(cmd *cobra.Command) {
		t.Run(cmd.CommandPath(), func(t *testing.T) {
			assert.False(t,
				strings.Contains(cmd.Long, "\nExample:") ||
					strings.Contains(cmd.Long, "\nExamples:"),
				"%s: Long contains embedded examples; move to Example field",
				cmd.CommandPath())
		})
	})
}

// TestAllFlagsHaveDescriptions verifies every registered flag across all
// commands has a non-empty usage description string.
func TestAllFlagsHaveDescriptions(t *testing.T) {
	walkCommands(rootCmd, func(cmd *cobra.Command) {
		cmd.Flags().VisitAll(func(f *pflag.Flag) {
			t.Run(cmd.CommandPath()+"/--"+f.Name, func(t *testing.T) {
				assert.NotEmpty(t, f.Usage,
					"flag --%s on %s has no description", f.Name, cmd.CommandPath())
			})
		})
	})
}

// TestCommandGroupsAssigned verifies all top-level commands (direct
// children of the root) have a GroupID set for organized help output.
func TestCommandGroupsAssigned(t *testing.T) {
	for _, cmd := range rootCmd.Commands() {
		if !cmd.IsAvailableCommand() {
			continue
		}
		t.Run(cmd.Name(), func(t *testing.T) {
			assert.NotEmpty(t, cmd.GroupID,
				"top-level command %q missing GroupID", cmd.Name())
		})
	}
}

// TestRootHelpContainsGroups verifies the root --help output shows the
// command groups rather than a flat "Available Commands:" list.
func TestRootHelpContainsGroups(t *testing.T) {
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetArgs([]string{"--help"})

	err := rootCmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "Wallet Operations:")
	assert.Contains(t, output, "Security & Access:")
	assert.Contains(t, output, "Configuration:")
}

// TestParentCommandsShowSubcommandsInHelp verifies that parent commands
// show their subcommands in the rendered help output via Cobra's built-in
// "Available Commands:" section.
func TestParentCommandsShowSubcommandsInHelp(t *testing.T) {
	parentCmds := []struct {
		name string
		cmd  *cobra.Command
	}{
		{"wallet", walletCmd},
		{"balance", balanceCmd},
		{"tx", txCmd},
		{"addresses", addressesCmd},
		{"utxo", utxoCmd},
		{"agent", agentCmd},
		{"session", sessionCmd},
		{"backup", backupCmd},
		{"config", configCmd},
	}

	for _, pc := range parentCmds {
		t.Run(pc.name, func(t *testing.T) {
			buf := new(bytes.Buffer)
			pc.cmd.SetOut(buf)
			require.NoError(t, pc.cmd.Help())
			helpOutput := buf.String()

			assert.Contains(t, helpOutput, "Available Commands:",
				"parent command %q missing Available Commands section", pc.name)

			// Verify each subcommand appears in the help output
			for _, sub := range pc.cmd.Commands() {
				if sub.IsAvailableCommand() {
					assert.Contains(t, helpOutput, sub.Name(),
						"parent %q missing subcommand %q in help", pc.name, sub.Name())
				}
			}
		})
	}
}

// TestLeafCommandHelpShowsExamplesSection verifies the rendered help
// output of a representative leaf command includes a labeled "Examples:"
// section from the Example field.
func TestLeafCommandHelpShowsExamplesSection(t *testing.T) {
	// Use wallet create as representative
	cmds := []*cobra.Command{
		walletCreateCmd,
		balanceShowCmd,
		txSendCmd,
		receiveCmd,
	}

	for _, cmd := range cmds {
		t.Run(cmd.CommandPath(), func(t *testing.T) {
			buf := new(bytes.Buffer)
			cmd.SetOut(buf)

			require.NoError(t, cmd.Help())
			helpOutput := buf.String()

			assert.Contains(t, helpOutput, "Examples:")
			assert.Contains(t, helpOutput, "sigil")
		})
	}
}

// TestWalkCommandsVisitsAll verifies walkCommands discovers every command.
func TestWalkCommandsVisitsAll(t *testing.T) {
	var visited []string
	walkCommands(rootCmd, func(cmd *cobra.Command) {
		visited = append(visited, cmd.CommandPath())
	})

	// Verify minimum expected commands exist
	expectedPaths := []string{
		"sigil",
		"sigil wallet",
		"sigil wallet create",
		"sigil wallet list",
		"sigil wallet show",
		"sigil wallet restore",
		"sigil wallet discover",
		"sigil balance",
		"sigil balance show",
		"sigil tx",
		"sigil tx send",
		"sigil receive",
		"sigil addresses",
		"sigil addresses list",
		"sigil utxo",
		"sigil agent",
		"sigil session",
		"sigil backup",
		"sigil config",
		"sigil completion",
		"sigil version",
	}

	for _, expected := range expectedPaths {
		assert.Contains(t, visited, expected,
			"walkCommands did not visit %q", expected)
	}
}

// newTestTree creates a minimal root -> parent -> children tree so that
// Cobra's IsAvailableCommand() returns true for children.
// newNoopRun returns a no-op Run function to make test commands "runnable" in Cobra.
func newNoopRun() func(*cobra.Command, []string) {
	return func(_ *cobra.Command, _ []string) {}
}

// TestEnrichParentLong verifies the enrichment function appends a correct
// subcommand list to a parent command.
func TestEnrichParentLong(t *testing.T) {
	parent := &cobra.Command{Use: "parent", Short: "Parent", Long: "Base description."}
	child1 := &cobra.Command{Use: "sub1", Short: "First subcommand", Run: newNoopRun()}
	child2 := &cobra.Command{Use: "sub2", Short: "Second subcommand", Run: newNoopRun()}
	parent.AddCommand(child1, child2)

	enrichParentLong(parent)

	assert.Contains(t, parent.Long, "Base description.")
	assert.Contains(t, parent.Long, "Subcommands:")
	assert.Contains(t, parent.Long, "sub1")
	assert.Contains(t, parent.Long, "First subcommand")
	assert.Contains(t, parent.Long, "sub2")
	assert.Contains(t, parent.Long, "Second subcommand")
}

// TestEnrichParentLong_NoSubcommands verifies enrichment is a no-op for
// leaf commands.
func TestEnrichParentLong_NoSubcommands(t *testing.T) {
	leaf := &cobra.Command{
		Use:   "leaf",
		Short: "A leaf",
		Long:  "Leaf description.",
	}

	enrichParentLong(leaf)

	assert.Equal(t, "Leaf description.", leaf.Long)
}

// TestEnrichParentLong_HiddenSubcommands verifies hidden subcommands are
// excluded from the dynamic subcommand list.
func TestEnrichParentLong_HiddenSubcommands(t *testing.T) {
	parent := &cobra.Command{Use: "parent", Short: "Parent", Long: "Parent desc."}
	visible := &cobra.Command{Use: "visible", Short: "Visible command", Run: newNoopRun()}
	hidden := &cobra.Command{Use: "hidden", Short: "Hidden command", Hidden: true, Run: newNoopRun()}
	parent.AddCommand(visible, hidden)

	enrichParentLong(parent)

	assert.Contains(t, parent.Long, "visible")
	assert.NotContains(t, parent.Long, "hidden")
}

// TestCommandShortDescriptionsAreReasonableLength verifies Short
// descriptions are concise (under 80 chars) for clean help output.
func TestCommandShortDescriptionsAreReasonableLength(t *testing.T) {
	const maxShortLen = 80

	walkCommands(rootCmd, func(cmd *cobra.Command) {
		t.Run(cmd.CommandPath(), func(t *testing.T) {
			assert.LessOrEqual(t, len(cmd.Short), maxShortLen,
				"%s: Short description too long (%d chars): %q",
				cmd.CommandPath(), len(cmd.Short), cmd.Short)
		})
	})
}

// TestExamplesContainCommandName verifies that Example fields reference
// the actual sigil command for clarity.
func TestExamplesContainCommandName(t *testing.T) {
	walkCommands(rootCmd, func(cmd *cobra.Command) {
		if cmd.Example == "" {
			return
		}
		t.Run(cmd.CommandPath(), func(t *testing.T) {
			assert.Contains(t, cmd.Example, "sigil",
				"%s: Example should contain 'sigil' to show full command invocation",
				cmd.CommandPath())
		})
	})
}

// TestRequiredFlagsDocumented verifies that flags marked as required
// include "(required)" in their usage description or the command's
// required annotation matches.
func TestRequiredFlagsDocumented(t *testing.T) {
	walkCommands(rootCmd, func(cmd *cobra.Command) {
		annotations := cmd.Annotations
		_ = annotations // Cobra stores required flag names internally

		cmd.Flags().VisitAll(func(f *pflag.Flag) {
			// Check if flag is in the required list
			requiredAnnotation := f.Annotations
			if requiredAnnotation == nil {
				return
			}
			if _, isRequired := requiredAnnotation[cobra.BashCompOneRequiredFlag]; !isRequired {
				return
			}

			t.Run(cmd.CommandPath()+"/--"+f.Name, func(t *testing.T) {
				assert.Contains(t, f.Usage, "required",
					"required flag --%s on %s should mention 'required' in its description",
					f.Name, cmd.CommandPath())
			})
		})
	})
}

// TestMutuallyExclusiveFlagsOnReceive verifies that Cobra's flag
// constraints are properly set on the receive command.
func TestMutuallyExclusiveFlagsOnReceive(t *testing.T) {
	// Mark both --check and --new as changed to trigger mutual exclusion
	require.NoError(t, receiveCmd.Flags().Set("check", "true"))
	require.NoError(t, receiveCmd.Flags().Set("new", "true"))
	t.Cleanup(func() {
		receiveCheck = false
		receiveNew = false
	})

	err := receiveCmd.ValidateFlagGroups()
	require.Error(t, err, "expected error for --check and --new together")
	assert.Contains(t, err.Error(), "none of the others can be")
}

// TestMutuallyExclusiveAddressAllOnReceive verifies --address and --all
// are declared mutually exclusive on the receive command.
func TestMutuallyExclusiveAddressAllOnReceive(t *testing.T) {
	require.NoError(t, receiveCmd.Flags().Set("address", "1ABC"))
	require.NoError(t, receiveCmd.Flags().Set("all", "true"))
	t.Cleanup(func() {
		receiveAddress = ""
		receiveAll = false
	})

	err := receiveCmd.ValidateFlagGroups()
	require.Error(t, err, "expected error for --address and --all together")
	assert.Contains(t, err.Error(), "none of the others can be")
}

// TestMutuallyExclusiveFlagsOnAgentRevoke verifies that Cobra's flag
// constraints are properly set on the agent revoke command.
func TestMutuallyExclusiveFlagsOnAgentRevoke(t *testing.T) {
	// Set both --id and --all as changed
	require.NoError(t, agentRevokeCmd.Flags().Set("id", "agt_test"))
	require.NoError(t, agentRevokeCmd.Flags().Set("all", "true"))
	t.Cleanup(func() {
		agentID = ""
		agentRevokeAll = false
	})

	err := agentRevokeCmd.ValidateFlagGroups()
	require.Error(t, err, "expected error for --id and --all together")
	assert.Contains(t, err.Error(), "none of the others can be")
}

// TestOneRequiredFlagsOnAgentRevoke verifies that at least one of
// --id or --all must be provided on the agent revoke command.
func TestOneRequiredFlagsOnAgentRevoke(t *testing.T) {
	// Ensure neither --id nor --all is set by creating a fresh parse
	// We need the flags to not be "changed"
	agentID = ""
	agentRevokeAll = false

	// Reset the Changed state by visiting flags
	agentRevokeCmd.Flags().VisitAll(func(f *pflag.Flag) {
		if f.Name == "id" || f.Name == "all" {
			f.Changed = false
		}
	})

	err := agentRevokeCmd.ValidateFlagGroups()
	require.Error(t, err, "expected error when neither --id nor --all is set")
}

// TestHelpOutputContainsGlobalFlags verifies the rendered help for a
// leaf command includes inherited global flags.
func TestHelpOutputContainsGlobalFlags(t *testing.T) {
	buf := new(bytes.Buffer)
	walletCreateCmd.SetOut(buf)
	_ = walletCreateCmd.Help()
	output := buf.String()

	assert.Contains(t, output, "--home")
	assert.Contains(t, output, "--output")
	assert.Contains(t, output, "--verbose")
}

// TestCommandUseLinesAreSet verifies every command has a Use field.
func TestCommandUseLinesAreSet(t *testing.T) {
	walkCommands(rootCmd, func(cmd *cobra.Command) {
		t.Run(cmd.CommandPath(), func(t *testing.T) {
			assert.NotEmpty(t, cmd.Use,
				"%s: missing Use field", cmd.CommandPath())
		})
	})
}
