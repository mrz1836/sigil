package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// walkCommands visits every command in the tree depth-first.
func walkCommands(cmd *cobra.Command, fn func(*cobra.Command)) {
	fn(cmd)
	for _, sub := range cmd.Commands() {
		walkCommands(sub, fn)
	}
}

// enrichParentLong appends a dynamically generated subcommand list to a parent
// command's Long description. This ensures parent help stays current when
// subcommands are added or removed.
func enrichParentLong(cmd *cobra.Command) {
	if !cmd.HasSubCommands() {
		return
	}

	var sb strings.Builder
	sb.WriteString(cmd.Long)
	sb.WriteString("\n\nSubcommands:\n")

	for _, sub := range cmd.Commands() {
		if sub.IsAvailableCommand() {
			sb.WriteString(fmt.Sprintf("  %-16s %s\n", sub.Name(), sub.Short))
		}
	}

	cmd.Long = sb.String()
}
