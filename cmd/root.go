package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func Execute() error {
	return executeRootCommand(newRootCmd(), os.Args[1:])
}

func newBaseRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:           "oa",
		Short:         "OpenAI Accounts CLI (oa): manage auth and usage limits",
		Long:          "oa (OpenAI Accounts CLI) helps you store account auth references, run OpenAI login flows, fetch usage/limit snapshots, and view account status from the terminal.",
		SilenceUsage:  true,
		SilenceErrors: false,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if wantsJSON(cmd) {
				return writeJSONError(cmd, fmt.Errorf("%s: subcommand required", cmd.CommandPath()))
			}
			return cmd.Help()
		},
	}

	rootCmd.PersistentFlags().Bool("json", false, "Render JSON output")

	return rootCmd
}

func newRootCmd() *cobra.Command {
	app, err := wireApp()
	if err != nil {
		rootCmd := newBaseRootCmd()

		rootCmd.RunE = func(cmd *cobra.Command, _ []string) error {
			return writeJSONError(cmd, err)
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
