package cli

import (
	selfupdate "github.com/mrz1836/go-selfupdate"
	"github.com/mrz1836/go-selfupdate/cobracmd"
	"github.com/spf13/cobra"
)

// attachUpdateCommand registers sigil's self-update command on root and wires
// the passive "a new version is available" notice, both derived from a single
// self-update config. The running binary's compiled-in version is threaded in
// explicitly so a binary run from outside PATH still updates itself, and the
// registered command is returned so the caller can slot it into the help groups.
func attachUpdateCommand(root *cobra.Command, version string) *cobra.Command {
	// One call registers the `update` command (alias `upgrade`, flags
	// --check/--force/--verbose) and the passive "a new version is available"
	// banner, both derived from this single config. sigil installs only from its
	// GitHub release archives — verifying the SHA-256 checksum and atomically
	// replacing the binary — and refuses to overwrite a binary owned by
	// `go install` or Homebrew. AppName derives from BinaryName, giving the
	// SIGIL_ env prefix (opt out with SIGIL_NO_UPDATE_CHECK; the shared
	// NO_UPDATE_CHECK and CI also disable it). The deprecated --use-binary flag
	// is kept, hidden and inert, so old invocations do not error now that a
	// release archive is the only install route.
	cmd := cobracmd.Attach(root, selfupdate.Config{ //nolint:gosec // G101 false positive: TokenEnvVar is an environment variable name, not a credential
		Owner:          "mrz1836",
		Repo:           "sigil",
		BinaryName:     "sigil",
		CurrentVersion: version,
		TokenEnvVar:    "SIGIL_GITHUB_TOKEN",
	}, cobracmd.WithDeprecatedUseBinaryFlag())

	// Match sigil's help conventions: grouped output and a rendered Examples
	// section, like every other command in the tree.
	cmd.GroupID = "config"
	cmd.Example = `  sigil update
  sigil update --check
  sigil update --force`

	return cmd
}
