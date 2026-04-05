package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/bnema/openai-accounts-cli/internal/application"
	"github.com/bnema/openai-accounts-cli/internal/domain"
)

const (
	opencodeProviderID   = "openai"
	opencodePluginRelDir = ".config/opencode/plugins"
	opencodePluginName   = "oa-plugin.js"
	opencodeAuthRelPath  = ".local/share/opencode/auth.json"
)

type opencodeAuthEntry struct {
	Type      string `json:"type"`
	Refresh   string `json:"refresh"`
	Access    string `json:"access"`
	Expires   int64  `json:"expires"`
	AccountID string `json:"accountId,omitempty"`
}

func opencodePluginPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}

	return filepath.Join(homeDir, opencodePluginRelDir, opencodePluginName), nil
}

func opencodeAuthPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}

	return filepath.Join(homeDir, opencodeAuthRelPath), nil
}

func loadOAuthTokensForAccount(ctx context.Context, app *app, accountID domain.AccountID) (oauthTokens, application.Status, error) {
	status, err := app.service.GetStatus(ctx, accountID)
	if err != nil {
		return oauthTokens{}, application.Status{}, err
	}
	if status.Account.Auth.SecretRef == "" {
		return oauthTokens{}, application.Status{}, fmt.Errorf("account [%s] %s: missing oauth secret ref", status.Account.ID, status.Account.Name)
	}
	secretValue, err := app.secretStore.Get(ctx, status.Account.Auth.SecretRef)
	if err != nil {
		return oauthTokens{}, application.Status{}, fmt.Errorf("account [%s] %s: load secret: %w", status.Account.ID, status.Account.Name, err)
	}
	tokens, err := decodeOAuthTokens(secretValue)
	if err != nil {
		return oauthTokens{}, application.Status{}, fmt.Errorf("account [%s] %s: decode tokens: %w", status.Account.ID, status.Account.Name, err)
	}
	return tokens, status, nil
}

func syncAccountIntoOpencode(ctx context.Context, app *app, accountID domain.AccountID) (application.Status, error) {
	tokens, status, err := loadOAuthTokensForAccount(ctx, app, accountID)
	if err != nil {
		return application.Status{}, err
	}

	var expiresMs int64
	if tokens.ExpiresAt > 0 {
		expiresMs = tokens.ExpiresAt * 1000
	} else if tokens.ExpiresIn > 0 {
		expiresMs = app.now().Add(time.Duration(tokens.ExpiresIn) * time.Second).UnixMilli()
	}

	entry := opencodeAuthEntry{Type: "oauth", Refresh: tokens.RefreshToken, Access: tokens.AccessToken, Expires: expiresMs, AccountID: accountIDFromToken(tokens.IDToken)}
	authPath, err := opencodeAuthPath()
	if err != nil {
		return application.Status{}, err
	}
	if err := writeOpencodeAuthEntry(authPath, opencodeProviderID, entry); err != nil {
		return application.Status{}, err
	}
	return status, nil
}

func writeOpencodeAuthEntry(path, provider string, entry opencodeAuthEntry) error {
	raw := map[string]json.RawMessage{}

	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
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
