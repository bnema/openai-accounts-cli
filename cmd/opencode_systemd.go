package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

func newOpencodeInstallSystemdCmd(_ *app) *cobra.Command {
	return &cobra.Command{
		Use:   "install-systemd",
		Short: "Install a systemd user timer for OpenCode sync",
		RunE: func(cmd *cobra.Command, _ []string) error {
			unitDir, err := opencodeSystemdUnitDir()
			if err != nil {
				return err
			}
			if err := os.MkdirAll(unitDir, 0o700); err != nil {
				return fmt.Errorf("create systemd user unit directory: %w", err)
			}

			execPath, err := os.Executable()
			if err != nil {
				return fmt.Errorf("resolve executable path: %w", err)
			}

			servicePath := filepath.Join(unitDir, opencodeServiceName)
			timerPath := filepath.Join(unitDir, opencodeTimerName)
			if err := os.WriteFile(servicePath, []byte(renderOpencodeSystemdService(execPath)), 0o600); err != nil {
				return fmt.Errorf("write systemd service unit: %w", err)
			}
			if err := os.WriteFile(timerPath, []byte(renderOpencodeSystemdTimer()), 0o600); err != nil {
				return fmt.Errorf("write systemd timer unit: %w", err)
			}

			_, err = fmt.Fprintf(cmd.OutOrStdout(), "installed %s and %s in %s\nrun: systemctl --user daemon-reload && systemctl --user enable --now %s\n", opencodeServiceName, opencodeTimerName, unitDir, opencodeTimerName)
			return err
		},
	}
}

func renderOpencodeSystemdService(execPath string) string {
	return fmt.Sprintf("[Unit]\nDescription=Sync OpenCode auth with the best available oa account\n\n[Service]\nType=oneshot\nExecStart=%s opencode sync\n", systemdQuote(execPath))
}

func systemdQuote(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `\"`) + `"`
}

func renderOpencodeSystemdTimer() string {
	return "[Unit]\nDescription=Refresh OpenCode auth every 10 minutes\n\n[Timer]\nOnBootSec=2m\nOnUnitActiveSec=10m\nUnit=oa-opencode-sync.service\n\n[Install]\nWantedBy=timers.target\n"
}
