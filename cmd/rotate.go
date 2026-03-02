package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	statusrender "github.com/bnema/openai-accounts-cli/internal/adapters/render/status"
	"github.com/bnema/openai-accounts-cli/internal/application"
	"github.com/bnema/openai-accounts-cli/internal/domain"
	"github.com/spf13/cobra"
)

const (
	opencodeAuthProvider = "codex"
	rotateStaleThreshold = 6 * time.Hour
)

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

			// Fetch fresh limit data before picking the best account.
			statuses, err := app.service.GetStatusAll(ctx)
			if err != nil {
				return fmt.Errorf("load account statuses: %w", err)
			}

			chatgptAccounts := filterChatGPTAccounts(statuses)
			if len(chatgptAccounts) > 0 {
				fetchCmd := func(ctx context.Context) error {
					return fetchAccountsConcurrently(ctx, app, chatgptAccounts, cmd.ErrOrStderr())
				}
				if err := runUsageFetchSpinner(ctx, cmd.ErrOrStderr(), "Fetching usage limits...", fetchCmd); err != nil {
					return fmt.Errorf("fetch account limits: %w", err)
				}
				// Reload after fetch so priority uses fresh data.
				statuses, err = app.service.GetStatusAll(ctx)
				if err != nil {
					return fmt.Errorf("reload account statuses: %w", err)
				}
				now = app.now()
			}

			ordered := statusrender.PrioritizeStatuses(statuses, now)
			if len(ordered) == 0 {
				return fmt.Errorf("no accounts configured")
			}

			// Pick the first non-stale account that has valid credentials.
			var chosenStatus application.Status
			var chosenTokens oauthTokens
			var found bool
			var lastErr error

			for _, status := range ordered {
				// Skip accounts with stale limit data — their usage state is
				// unknown and they should not be rotated into active use.
				if isStatusStale(status, now, rotateStaleThreshold) {
					lastErr = fmt.Errorf("account [%s] %s: limit data is stale", status.Account.ID, status.Account.Name)
					continue
				}

				secretRef := status.Account.Auth.SecretRef
				if secretRef == "" {
					continue
				}

				secretValue, getErr := app.secretStore.Get(ctx, secretRef)
				if getErr != nil {
					lastErr = fmt.Errorf("account [%s] %s: load secret: %w", status.Account.ID, status.Account.Name, getErr)
					continue
				}

				tokens, decErr := decodeOAuthTokens(secretValue)
				if decErr != nil {
					lastErr = fmt.Errorf("account [%s] %s: decode tokens: %w", status.Account.ID, status.Account.Name, decErr)
					continue
				}

				chosenStatus = status
				chosenTokens = tokens
				found = true
				break
			}

			if !found {
				if lastErr != nil {
					return fmt.Errorf("no account with valid credentials found (last error: %w)", lastErr)
				}
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

// isStatusStale returns true if any available limit snapshot is older than maxAge.
// An account with no limit data at all is also considered stale.
func isStatusStale(status application.Status, now time.Time, maxAge time.Duration) bool {
	hasAny := false

	check := func(snapshot *application.StatusLimit) bool {
		if snapshot == nil {
			return false
		}
		hasAny = true
		return (domain.LimitSnapshot{AsOf: snapshot.CapturedAt}).IsStale(now, maxAge)
	}

	if check(status.DailyLimit) {
		return true
	}
	if check(status.WeeklyLimit) {
		return true
	}

	return !hasAny
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
	raw := map[string]json.RawMessage{}

	data, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("read opencode auth file: %w", err)
	}
	if err == nil {
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
