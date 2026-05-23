package cmd

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

//go:embed extensions/pi/oa-auth-hot-reload.ts
var piAuthHotReloadExtension string

func newInstallCmd(app *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install local tool integrations",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if wantsJSON(cmd) {
				return writeJSONError(cmd, fmt.Errorf("%s: subcommand required", cmd.CommandPath()))
			}
			return cmd.Help()
		},
	}

	cmd.AddCommand(
		newOpencodeInstallCmd(app),
		newPIInstallCmd(app),
	)
	return cmd
}

func newPIInstallCmd(_ *app) *cobra.Command {
	return &cobra.Command{
		Use:   "pi",
		Short: "Install Pi auth hot-reload extension",
		RunE: func(cmd *cobra.Command, _ []string) error {
			path, err := piExtensionPath()
			if err != nil {
				return writeJSONError(cmd, err)
			}
			if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
				return writeJSONError(cmd, fmt.Errorf("create pi extension directory: %w", err))
			}
			if err := os.WriteFile(path, []byte(piAuthHotReloadExtension), 0o600); err != nil {
				return writeJSONError(cmd, fmt.Errorf("write pi auth hot-reload extension: %w", err))
			}

			if wantsJSON(cmd) {
				return writeJSONOutput(cmd, map[string]any{"ok": true, "path": path, "reload_command": "/reload"})
			}

			_, err = fmt.Fprintf(cmd.OutOrStdout(), "installed Pi extension to %s\nreload active Pi sessions with /reload\n", path)
			return err
		},
	}
}

func newOpencodeInstallCmd(_ *app) *cobra.Command {
	return &cobra.Command{
		Use:   "opencode",
		Short: "Install OpenCode integration",
		RunE: func(cmd *cobra.Command, _ []string) error {
			configDir, err := opencodeConfigDir()
			if err != nil {
				return writeJSONError(cmd, err)
			}
			path, err := opencodePluginPath()
			if err != nil {
				return writeJSONError(cmd, err)
			}

			if err := os.MkdirAll(configDir, 0o700); err != nil {
				return writeJSONError(cmd, fmt.Errorf("create opencode config directory: %w", err))
			}
			if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
				return writeJSONError(cmd, fmt.Errorf("create opencode plugin directory: %w", err))
			}
			packagePath := filepath.Join(configDir, "package.json")
			if err := writeOpencodePluginPackage(packagePath); err != nil {
				return writeJSONError(cmd, err)
			}

			if err := os.WriteFile(path, []byte(opencodeShim), 0o600); err != nil {
				return writeJSONError(cmd, fmt.Errorf("write opencode plugin shim: %w", err))
			}

			if wantsJSON(cmd) {
				return writeJSONOutput(cmd, map[string]any{"ok": true, "path": path, "config_path": packagePath})
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
