package cmd

import (
	"context"
	"encoding/json"
	"errors"
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
	systemdUserRelDir    = ".config/systemd/user"
	opencodeServiceName  = "oa-opencode-sync.service"
	opencodeTimerName    = "oa-opencode-sync.timer"
)

type opencodeAuthEntry struct {
	Type      string `json:"type"`
	Refresh   string `json:"refresh"`
	Access    string `json:"access"`
	Expires   int64  `json:"expires"`
	AccountID string `json:"accountId,omitempty"`
}

var errOpencodeCandidateUnavailable = errors.New("opencode candidate unavailable")

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

func opencodeSystemdUnitDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}

	return filepath.Join(homeDir, systemdUserRelDir), nil
}

func loadOAuthTokensForAccount(ctx context.Context, app *app, accountID domain.AccountID) (oauthTokens, application.Status, error) {
	status, err := app.service.GetStatus(ctx, accountID)
	if err != nil {
		return oauthTokens{}, application.Status{}, err
	}
	if status.Account.Auth.SecretRef == "" {
		return oauthTokens{}, application.Status{}, fmt.Errorf("%w: account [%s] %s: missing oauth secret ref", errOpencodeCandidateUnavailable, status.Account.ID, status.Account.Name)
	}
	secretValue, err := app.secretStore.Get(ctx, status.Account.Auth.SecretRef)
	if err != nil {
		return oauthTokens{}, application.Status{}, fmt.Errorf("account [%s] %s: load secret: %w", status.Account.ID, status.Account.Name, err)
	}
	tokens, err := decodeOAuthTokens(secretValue)
	if err != nil {
		return oauthTokens{}, application.Status{}, fmt.Errorf("%w: account [%s] %s: decode tokens: %w", errOpencodeCandidateUnavailable, status.Account.ID, status.Account.Name, err)
	}
	if err := validateOpencodeTokens(tokens); err != nil {
		return oauthTokens{}, application.Status{}, fmt.Errorf("%w: account [%s] %s: %w", errOpencodeCandidateUnavailable, status.Account.ID, status.Account.Name, err)
	}
	return tokens, status, nil
}

func validateOpencodeTokens(tokens oauthTokens) error {
	if tokens.AccessToken == "" {
		return errors.New("missing access token")
	}
	if tokens.RefreshToken == "" {
		return errors.New("missing refresh token")
	}
	return nil
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
	fileMode := os.FileMode(0o600)

	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read opencode auth file: %w", err)
	}
	if info, statErr := os.Stat(path); statErr == nil {
		fileMode = info.Mode().Perm()
	} else if !os.IsNotExist(statErr) {
		return fmt.Errorf("stat opencode auth file: %w", statErr)
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

	tempFile, err := os.CreateTemp(filepath.Dir(path), ".auth.json.*")
	if err != nil {
		return fmt.Errorf("create temp opencode auth file: %w", err)
	}
	tempPath := tempFile.Name()
	defer func() {
		_ = os.Remove(tempPath)
	}()

	if err := tempFile.Chmod(fileMode); err != nil {
		_ = tempFile.Close()
		return fmt.Errorf("chmod temp opencode auth file: %w", err)
	}
	if _, err := tempFile.Write(out); err != nil {
		_ = tempFile.Close()
		return fmt.Errorf("write temp opencode auth file: %w", err)
	}
	if err := tempFile.Sync(); err != nil {
		_ = tempFile.Close()
		return fmt.Errorf("sync temp opencode auth file: %w", err)
	}
	if err := tempFile.Close(); err != nil {
		return fmt.Errorf("close temp opencode auth file: %w", err)
	}
	if err := os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("replace opencode auth file: %w", err)
	}

	return nil
}
