package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	statusrender "github.com/bnema/openai-accounts-cli/internal/adapters/render/status"
	"github.com/bnema/openai-accounts-cli/internal/application"
	"github.com/spf13/cobra"
)

const opencodeAuthProvider = "codex"

// opencodeAuthEntry matches the shape opencode expects in auth.json.
type opencodeAuthEntry struct {
	Type      string `json:"type"`
	Refresh   string `json:"refresh"`
	Access    string `json:"access"`
	Expires   int64  `json:"expires"` // unix milliseconds
	AccountID string `json:"accountId,omitempty"`
}

func newRotateCmd(app *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rotate",
		Short: "Rotate credentials for external tools",
	}

	cmd.AddCommand(newRotateOpencodeCmd(app))

	return cmd
}

func newRotateOpencodeCmd(app *app) *cobra.Command {
	return &cobra.Command{
		Use:   "opencode",
		Short: "Rotate opencode's codex provider with the best available account",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			now := app.now()

			statuses, err := app.service.GetStatusAll(ctx)
			if err != nil {
				return fmt.Errorf("load account statuses: %w", err)
			}

			ordered := statusrender.PrioritizeStatuses(statuses, now)
			if len(ordered) == 0 {
				return fmt.Errorf("no accounts configured")
			}

			// Find the first account that has auth credentials.
			var chosenStatus application.Status
			var chosenTokens oauthTokens
			var found bool
			for _, status := range ordered {
				secretRef := status.Account.Auth.SecretRef
				if secretRef == "" {
					continue
				}
				secretValue, getErr := app.secretStore.Get(ctx, secretRef)
				if getErr != nil {
					continue
				}
				tokens, decErr := decodeOAuthTokens(secretValue)
				if decErr != nil {
					continue
				}
				chosenStatus = status
				chosenTokens = tokens
				found = true
				break
			}

			if !found {
				return fmt.Errorf("no account with valid credentials found")
			}

			accountID := accountIDFromToken(chosenTokens.IDToken)

			// Convert expires_at (unix seconds) to milliseconds for opencode.
			var expiresMs int64
			if chosenTokens.ExpiresAt > 0 {
				expiresMs = chosenTokens.ExpiresAt * 1000
			} else if chosenTokens.ExpiresIn > 0 {
				expiresMs = now.Add(time.Duration(chosenTokens.ExpiresIn) * time.Second).UnixMilli()
			}

			entry := opencodeAuthEntry{
				Type:      "oauth",
				Refresh:   chosenTokens.RefreshToken,
				Access:    chosenTokens.AccessToken,
				Expires:   expiresMs,
				AccountID: accountID,
			}

			authPath, err := opencodeAuthPath()
			if err != nil {
				return err
			}

			if err := writeOpencodeAuthEntry(authPath, opencodeAuthProvider, entry); err != nil {
				return err
			}

			account := chosenStatus.Account
			label := account.Name
			if label == "" {
				label = string(account.ID)
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "rotated opencode codex auth to account [%s] %s\n", account.ID, label)
			return nil
		},
	}
}

func opencodeAuthPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	return filepath.Join(homeDir, ".local", "share", "opencode", "auth.json"), nil
}

// writeOpencodeAuthEntry reads auth.json, upserts the given provider key, and writes it back.
func writeOpencodeAuthEntry(path, provider string, entry opencodeAuthEntry) error {
	// Read existing file (tolerate missing).
	raw := map[string]json.RawMessage{}
	if data, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(data, &raw); err != nil {
			return fmt.Errorf("decode opencode auth file: %w", err)
		}
	}

	encoded, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("encode auth entry: %w", err)
	}
	raw[provider] = json.RawMessage(encoded)

	out, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return fmt.Errorf("encode opencode auth file: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create opencode auth directory: %w", err)
	}

	if err := os.WriteFile(path, out, 0o600); err != nil {
		return fmt.Errorf("write opencode auth file: %w", err)
	}

	return nil
}
