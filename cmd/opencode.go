package cmd

import "github.com/spf13/cobra"

func newOpencodeCmd(app *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "opencode",
		Short: "Manage OpenCode integration",
	}

	cmd.AddCommand(
		newOpencodeInstallSystemdCmd(app),
		newOpencodeDoctorCmd(app),
		newOpencodeSyncCmd(app),
	)

	return cmd
}
