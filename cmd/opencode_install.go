package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

func newOpencodeInstallCmd(_ *app) *cobra.Command {
	return &cobra.Command{
		Use:   "install",
		Short: "Install OpenCode integration",
		RunE: func(cmd *cobra.Command, _ []string) error {
			configDir, err := opencodeConfigDir()
			if err != nil {
				return err
			}
			path, err := opencodePluginPath()
			if err != nil {
				return err
			}

			if err := os.MkdirAll(configDir, 0o700); err != nil {
				return fmt.Errorf("create opencode config directory: %w", err)
			}
			if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
				return fmt.Errorf("create opencode plugin directory: %w", err)
			}
			if err := writeOpencodePluginPackage(filepath.Join(configDir, "package.json")); err != nil {
				return err
			}

			if err := os.WriteFile(path, []byte(opencodeShim), 0o600); err != nil {
				return fmt.Errorf("write opencode plugin shim: %w", err)
			}

			_, err = fmt.Fprintf(cmd.OutOrStdout(), "installed OpenCode plugin to %s\n", path)
			return err
		},
	}
}

func writeOpencodePluginPackage(path string) error {
	raw := map[string]json.RawMessage{}

	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read opencode package file: %w", err)
	}
	if err == nil {
		if err := json.Unmarshal(data, &raw); err != nil {
			return fmt.Errorf("decode opencode package file: %w", err)
		}
	}

	deps := map[string]string{}
	if existing, ok := raw["dependencies"]; ok {
		if err := json.Unmarshal(existing, &deps); err != nil {
			return fmt.Errorf("decode opencode package dependencies: %w", err)
		}
	}
	deps["@opencode-ai/plugin"] = "*"

	encodedDeps, err := json.Marshal(deps)
	if err != nil {
		return fmt.Errorf("encode opencode package dependencies: %w", err)
	}
	raw["dependencies"] = encodedDeps

	out, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return fmt.Errorf("encode opencode package file: %w", err)
	}

	if err := os.WriteFile(path, out, 0o600); err != nil {
		return fmt.Errorf("write opencode package file: %w", err)
	}

	return nil
}
