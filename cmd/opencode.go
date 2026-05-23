package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newOpencodeCmd(app *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "opencode",
		Short: "Manage OpenCode integration",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if wantsJSON(cmd) {
				return writeJSONError(cmd, fmt.Errorf("%s: subcommand required", cmd.CommandPath()))
			}
			return cmd.Help()
		},
	}

	cmd.AddCommand(
		newOpencodeInstallSystemdCmd(app),
		newOpencodeDoctorCmd(app),
		newOpencodeSyncCmd(app),
	)

	return cmd
}
