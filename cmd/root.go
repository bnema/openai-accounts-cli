package cmd

import "github.com/spf13/cobra"

func Execute() error {
	return newRootCmd().Execute()
}

func newBaseRootCmd() *cobra.Command {
	return &cobra.Command{
		Use:           "oa",
		Short:         "OpenAI Accounts CLI (oa): manage auth and usage limits",
		Long:          "oa (OpenAI Accounts CLI) helps you store account auth references, run OpenAI login flows, fetch usage/limit snapshots, and view account status from the terminal.",
		SilenceUsage:  true,
		SilenceErrors: false,
	}
}

func newRootCmd() *cobra.Command {
	app, err := wireApp()
	if err != nil {
		rootCmd := newBaseRootCmd()

		rootCmd.RunE = func(_ *cobra.Command, _ []string) error {
			return err
		}
		return rootCmd
	}

	return newRootCmdWithApp(app)
}

func newRootCmdWithApp(app *app) *cobra.Command {
	rootCmd := newBaseRootCmd()

	rootCmd.AddCommand(
		newVersionCmd(),
		newAccountCmd(app),
		newAuthCmd(app),
		newUsageCmd(app),
		newSyncCmd(app),
		newInstallCmd(app),
		newHandleCmd(app),
		newOpencodeCmd(app),
	)

	return rootCmd
}
