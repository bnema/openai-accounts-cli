package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

func newOpencodeInstallCmd(_ *app) *cobra.Command {
	return &cobra.Command{
		Use:   "install",
		Short: "Install OpenCode integration",
		RunE: func(_ *cobra.Command, _ []string) error {
			path, err := opencodePluginPath()
			if err != nil {
				return err
			}

			if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
				return fmt.Errorf("create opencode plugin directory: %w", err)
			}

			if err := os.WriteFile(path, []byte(opencodeShim), 0o600); err != nil {
				return fmt.Errorf("write opencode plugin shim: %w", err)
			}

			return nil
		},
	}
}
