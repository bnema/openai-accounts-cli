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
	opencodeConfigRelDir = ".config/opencode"
	opencodeProviderID   = "openai"
	opencodePluginRelDir = ".config/opencode/plugins"
	opencodePluginName   = "oa-plugin.js"
	opencodeAuthRelPath  = ".local/share/opencode/auth.json"
	codexAuthRelPath     = ".codex/auth.json"
	piAuthRelPath        = ".pi/agent/auth.json"
	piExtensionRelPath   = ".pi/agent/extensions/oa-auth-hot-reload.ts"
	piProviderID         = "openai-codex"
	systemdUserRelDir    = ".config/systemd/user"
	opencodeServiceName  = "oa-opencode-sync.service"
	opencodeTimerName    = "oa-opencode-sync.timer"
)

type oauthSyncEntry struct {
	Type      string `json:"type"`
	Refresh   string `json:"refresh"`
	Access    string `json:"access"`
	Expires   int64  `json:"expires"`
	AccountID string `json:"accountId,omitempty"`
}

type opencodeAuthEntry = oauthSyncEntry

var errSyncCandidateUnavailable = errors.New("sync candidate unavailable")

func opencodePluginPath() (string, error) {
	return opencodeHomeJoin(opencodePluginRelDir, opencodePluginName)
}

func opencodeConfigDir() (string, error) {
	return opencodeHomeJoin(opencodeConfigRelDir)
}

func opencodeAuthPath() (string, error) {
	return opencodeHomeJoin(opencodeAuthRelPath)
}

func codexAuthPath() (string, error) {
	return opencodeHomeJoin(codexAuthRelPath)
}

func piAuthPath() (string, error) {
	return opencodeHomeJoin(piAuthRelPath)
}

func piExtensionPath() (string, error) {
	return opencodeHomeJoin(piExtensionRelPath)
}

func opencodeSystemdUnitDir() (string, error) {
	return opencodeHomeJoin(systemdUserRelDir)
}

func opencodeHomeJoin(parts ...string) (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}

	return filepath.Join(append([]string{homeDir}, parts...)...), nil
}

func loadOAuthTokensForAccount(ctx context.Context, app *app, accountID domain.AccountID) (oauthTokens, application.Status, error) {
	status, err := app.service.GetStatus(ctx, accountID)
	if err != nil {
		return oauthTokens{}, application.Status{}, err
	}
	if status.Account.Auth.SecretRef == "" {
		return oauthTokens{}, application.Status{}, fmt.Errorf("%w: account [%s] %s: missing oauth secret ref", errSyncCandidateUnavailable, status.Account.ID, status.Account.Name)
	}
	secretValue, err := app.secretStore.Get(ctx, status.Account.Auth.SecretRef)
	if err != nil {
		return oauthTokens{}, application.Status{}, fmt.Errorf("account [%s] %s: load secret: %w", status.Account.ID, status.Account.Name, err)
	}
	tokens, err := decodeOAuthTokens(secretValue)
	if err != nil {
		return oauthTokens{}, application.Status{}, fmt.Errorf("%w: account [%s] %s: decode tokens: %w", errSyncCandidateUnavailable, status.Account.ID, status.Account.Name, err)
	}
	if err := validateSyncTokens(tokens); err != nil {
		return oauthTokens{}, application.Status{}, fmt.Errorf("%w: account [%s] %s: %w", errSyncCandidateUnavailable, status.Account.ID, status.Account.Name, err)
	}
	return tokens, status, nil
}

func validateSyncTokens(tokens oauthTokens) error {
	if tokens.AccessToken == "" {
		return errors.New("missing access token")
	}
	if tokens.RefreshToken == "" {
		return errors.New("missing refresh token")
	}
	if tokens.ExpiresAt <= 0 && tokens.ExpiresIn <= 0 {
		return errors.New("missing token expiry")
	}
	return nil
}

func oauthExpiryMillis(app *app, tokens oauthTokens) int64 {
	if tokens.ExpiresAt > 0 {
		return tokens.ExpiresAt * 1000
	}
	if tokens.ExpiresIn > 0 {
		return app.now().Add(time.Duration(tokens.ExpiresIn) * time.Second).UnixMilli()
	}
	return 0
}

func writeOpencodeAuthEntry(path, provider string, entry oauthSyncEntry) error {
	return writeJSONMapEntry(path, "opencode auth file", provider, entry)
}

func writePIAuthEntry(path, provider string, entry oauthSyncEntry) error {
	return writeJSONMapEntry(path, "pi auth file", provider, entry)
}

func writeJSONMapEntry(path, fileLabel, provider string, entry any) error {
	raw := map[string]json.RawMessage{}

	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read %s: %w", fileLabel, err)
	}
	if err == nil {
		if err := json.Unmarshal(data, &raw); err != nil {
			return fmt.Errorf("decode %s: %w", fileLabel, err)
		}
	}

	encoded, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("encode auth entry: %w", err)
	}
	raw[provider] = json.RawMessage(encoded)

	out, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return fmt.Errorf("encode %s: %w", fileLabel, err)
	}

	return writeJSONFileAtomic(path, fileLabel, out)
}

func writeJSONFileAtomic(path, fileLabel string, out []byte) error {
	fileMode, err := existingFileMode(path)
	if err != nil {
		return fmt.Errorf("stat %s: %w", fileLabel, err)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create %s directory: %w", fileLabel, err)
	}

	tempFile, err := os.CreateTemp(filepath.Dir(path), ".auth.json.*")
	if err != nil {
		return fmt.Errorf("create temp %s: %w", fileLabel, err)
	}
	tempPath := tempFile.Name()
	defer func() {
		_ = os.Remove(tempPath)
	}()

	if err := tempFile.Chmod(fileMode); err != nil {
		_ = tempFile.Close()
		return fmt.Errorf("chmod temp %s: %w", fileLabel, err)
	}
	if _, err := tempFile.Write(out); err != nil {
		_ = tempFile.Close()
		return fmt.Errorf("write temp %s: %w", fileLabel, err)
	}
	if err := tempFile.Sync(); err != nil {
		_ = tempFile.Close()
		return fmt.Errorf("sync temp %s: %w", fileLabel, err)
	}
	if err := tempFile.Close(); err != nil {
		return fmt.Errorf("close temp %s: %w", fileLabel, err)
	}
	if err := os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("replace %s: %w", fileLabel, err)
	}

	return nil
}

func existingFileMode(path string) (os.FileMode, error) {
	if info, err := os.Stat(path); err == nil {
		return info.Mode().Perm(), nil
	} else if !os.IsNotExist(err) {
		return 0, err
	}

	return 0o600, nil
}

func oauthSyncEntryFromTokens(app *app, tokens oauthTokens) oauthSyncEntry {
	return oauthSyncEntry{
		Type:      "oauth",
		Refresh:   tokens.RefreshToken,
		Access:    tokens.AccessToken,
		Expires:   oauthExpiryMillis(app, tokens),
		AccountID: accountIDFromToken(tokens.IDToken),
	}
}

func writeOAuthTokensToOpencode(app *app, tokens oauthTokens) error {
	authPath, err := opencodeAuthPath()
	if err != nil {
		return err
	}

	return writeOpencodeAuthEntry(authPath, opencodeProviderID, oauthSyncEntryFromTokens(app, tokens))
}

func writeOAuthTokensToCodex(app *app, tokens oauthTokens) error {
	type codexTokenData struct {
		IDToken      string `json:"id_token"`
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		AccountID    string `json:"account_id,omitempty"`
	}

	authPath, err := codexAuthPath()
	if err != nil {
		return err
	}

	raw := map[string]json.RawMessage{}
	data, err := os.ReadFile(authPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read codex auth file: %w", err)
	}
	if err == nil {
		if err := json.Unmarshal(data, &raw); err != nil {
			return fmt.Errorf("decode codex auth file: %w", err)
		}
	}

	if err := setJSONRawField(raw, "auth_mode", "chatgpt"); err != nil {
		return fmt.Errorf("encode codex auth mode: %w", err)
	}
	if err := setJSONRawField(raw, "OPENAI_API_KEY", (*string)(nil)); err != nil {
		return fmt.Errorf("encode codex api key: %w", err)
	}
	if err := setJSONRawField(raw, "tokens", codexTokenData{
		IDToken:      tokens.IDToken,
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
		AccountID:    accountIDFromToken(tokens.IDToken),
	}); err != nil {
		return fmt.Errorf("encode codex tokens: %w", err)
	}
	if err := setJSONRawField(raw, "last_refresh", app.now().UTC()); err != nil {
		return fmt.Errorf("encode codex last refresh: %w", err)
	}
	delete(raw, "openai_api_key")

	out, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return fmt.Errorf("encode codex auth file: %w", err)
	}

	return writeJSONFileAtomic(authPath, "codex auth file", out)
}

func setJSONRawField(raw map[string]json.RawMessage, key string, value any) error {
	encoded, err := json.Marshal(value)
	if err != nil {
		return err
	}
	raw[key] = json.RawMessage(encoded)
	return nil
}

func writeOAuthTokensToPI(app *app, tokens oauthTokens) error {
	authPath, err := piAuthPath()
	if err != nil {
		return err
	}

	return writePIAuthEntry(authPath, piProviderID, oauthSyncEntryFromTokens(app, tokens))
}
