package cmd

import "github.com/spf13/cobra"

func newOpencodeCmd(app *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "opencode",
		Short: "Manage OpenCode integration",
	}

	cmd.AddCommand(
		newOpencodeInstallCmd(app),
		newOpencodeHandleCmd(app),
		newOpencodeDoctorCmd(app),
		newOpencodeSyncCmd(app),
	)

	return cmd
}
func newOpencodeHandleCmd(_ *app) *cobra.Command {
	cmd := newNotImplementedCmd("handle", "Handle OpenCode requests")
	cmd.Flags().Bool("json", false, "Output JSON")
	return cmd
}
