package cli

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAttachUpdateCommandWiring locks the seam between the root command and the
// go-selfupdate cobracmd package: the self-update command is registered under
// the name "update" with the "upgrade" alias (preserving the old command name),
// the check/force/verbose boolean flags, and a hidden, inert --use-binary flag.
// The command's behavior itself is covered by the library's own suites, so this
// asserts only the wiring.
func TestAttachUpdateCommandWiring(t *testing.T) {
	t.Parallel()

	root := &cobra.Command{Use: "sigil"}
	root.AddGroup(&cobra.Group{ID: "config", Title: "Configuration:"})

	cmd := attachUpdateCommand(root, "v1.2.3")
	require.NotNil(t, cmd, "attachUpdateCommand returns the registered command")

	assert.Equal(t, "update", cmd.Name(), "the self-update command is named update")
	assert.Contains(t, cmd.Aliases, "upgrade", "the update command keeps the upgrade alias")
	assert.Equal(t, "config", cmd.GroupID, "the update command joins the Configuration group")
	assert.NotEmpty(t, cmd.Short, "the update command has a Short description")
	assert.NotEmpty(t, cmd.Long, "the update command has a Long description")
	assert.NotEmpty(t, cmd.Example, "the update command has an Example section")

	// The command is registered on root under the update name.
	var registered *cobra.Command
	for _, c := range root.Commands() {
		if c.Name() == "update" {
			registered = c
			break
		}
	}
	require.NotNil(t, registered, "root registers an update command")

	for _, name := range []string{"check", "force", "verbose"} {
		flag := registered.Flags().Lookup(name)
		require.NotNilf(t, flag, "the update command registers --%s", name)
		assert.Equalf(t, "bool", flag.Value.Type(), "--%s is a boolean flag", name)
	}

	// The deprecated --use-binary flag is accepted for compatibility but hidden.
	useBinary := registered.Flags().Lookup("use-binary")
	require.NotNil(t, useBinary, "the deprecated --use-binary flag is registered")
	assert.True(t, useBinary.Hidden, "--use-binary is hidden")
}
