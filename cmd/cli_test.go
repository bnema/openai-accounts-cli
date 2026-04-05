package cmd

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuthSetRequiresSecretValueFlag(t *testing.T) {
	home := t.TempDir()
	require.NoError(t, writeAccountsFixture(home))

	_, _, err := executeCLI(t, home,
		"auth", "set",
		"--account", "acc-1",
		"--method", "api_key",
		"--secret-key", "openai://acc-1/api_key",
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "required flag(s) \"secret-value\" not set")
}

func TestStatusByAccountHappyPath(t *testing.T) {
	home := t.TempDir()
	require.NoError(t, writeAccountsFixture(home))

	stdout, _, err := executeCLI(t, home, "status", "--account", "acc-1")
	require.NoError(t, err)
	assert.Contains(t, stdout, "accounts: 1")
	assert.Contains(t, stdout, "Primary (acc-1)")
}

func TestStatusByAccountJSONOutput(t *testing.T) {
	home := t.TempDir()
	require.NoError(t, writeAccountsFixture(home))

	stdout, _, err := executeCLI(t, home, "status", "--account", "acc-1", "--json")
	require.NoError(t, err)
	assert.True(t, json.Valid([]byte(stdout)))
	assert.Contains(t, stdout, "\"Account\"")
	assert.Contains(t, stdout, "\"ID\": \"acc-1\"")
}

func TestAuthSetThenStatusShowsAuthMethod(t *testing.T) {
	home := t.TempDir()
	require.NoError(t, writeAccountsFixture(home))

	_, _, err := executeCLI(t, home,
		"auth", "set",
		"--account", "acc-1",
		"--method", "api_key",
		"--secret-key", "openai://acc-1/api_key",
		"--secret-value", "test-secret-value",
	)
	require.NoError(t, err)

	stdout, _, err := executeCLI(t, home, "status", "--account", "acc-1")
	require.NoError(t, err)
	assert.Contains(t, stdout, "Primary (acc-1)")
}

func TestAuthSetAutoAssignsNextNumericAccountID(t *testing.T) {
	home := t.TempDir()
	require.NoError(t, writeAccountsFixture(home))

	_, _, err := executeCLI(t, home,
		"auth", "set",
		"--method", "api_key",
		"--secret-key", "openai://1/api_key",
		"--secret-value", "secret-1",
	)
	require.NoError(t, err)

	_, _, err = executeCLI(t, home,
		"auth", "set",
		"--method", "api_key",
		"--secret-key", "openai://2/api_key",
		"--secret-value", "secret-2",
	)
	require.NoError(t, err)

	stdout, _, err := executeCLI(t, home, "status")
	require.NoError(t, err)
	assert.Contains(t, stdout, "Account 1 (1)")
	assert.Contains(t, stdout, "Account 2 (2)")
}

func TestLoginDeviceReturnsNotImplemented(t *testing.T) {
	home := t.TempDir()
	require.NoError(t, writeAccountsFixture(home))

	_, _, err := executeCLI(t, home, "auth", "login", "device")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not implemented yet")
}

func TestLimitCommandIsRemoved(t *testing.T) {
	home := t.TempDir()
	require.NoError(t, writeAccountsFixture(home))

	_, _, err := executeCLI(t, home, "limit")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown command \"limit\"")
}

func TestRootCommandIncludesOpencodeCommands(t *testing.T) {
	root := newRootCmd()
	cmd, _, err := root.Find([]string{"opencode", "install"})
	require.NoError(t, err)
	assert.Equal(t, "install", cmd.Name())
}

func TestOpencodeHandleReturnsRetryDecisionJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/oauth/token":
			_, _ = fmt.Fprint(w, `{"access_token":"fresh-access-token","refresh_token":"refresh-token-456","id_token":"","token_type":"Bearer","expires_in":3600}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	t.Setenv("OA_AUTH_ISSUER", server.URL)
	t.Setenv("OA_AUTH_CLIENT_ID", "test-client-id")

	home := t.TempDir()
	require.NoError(t, writeAccountsFixture(home))

	_, _, err := executeCLI(t, home,
		"auth", "set",
		"--account", "acc-1",
		"--method", "chatgpt",
		"--secret-key", "openai://acc-1/oauth_tokens",
		"--secret-value", `{"access_token":"access-token-123","refresh_token":"refresh-token-123","id_token":"","expires_at":1890000000}`,
	)
	require.NoError(t, err)

	stdin := `{"provider":"openai","status":401,"message":"token expired","account_id":"acc-1"}`
	stdout, _, err := executeCLIWithStdin(t, home, stdin, "opencode", "handle", "--json")
	require.NoError(t, err)
	assert.Contains(t, stdout, `"action":"refresh_current"`)
	assert.Contains(t, stdout, `"retry_safe":true`)
	assert.Contains(t, stdout, `"access":"fresh-access-token"`)
}

func TestOpencodeHandleReturnsFallbackJSONWhenRefreshCurrentFails(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/oauth/token":
			w.WriteHeader(http.StatusBadGateway)
			_, _ = fmt.Fprint(w, `{"error":"temporarily_unavailable"}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	t.Setenv("OA_AUTH_ISSUER", server.URL)
	t.Setenv("OA_AUTH_CLIENT_ID", "test-client-id")

	home := t.TempDir()
	require.NoError(t, writeAccountsFixture(home))

	_, _, err := executeCLI(t, home,
		"auth", "set",
		"--account", "acc-1",
		"--method", "chatgpt",
		"--secret-key", "openai://acc-1/oauth_tokens",
		"--secret-value", `{"access_token":"stale-access-token","refresh_token":"refresh-token-123","id_token":"","expires_at":1}`,
	)
	require.NoError(t, err)

	stdin := `{"provider":"openai","status":401,"message":"token expired","account_id":"acc-1"}`
	stdout, _, err := executeCLIWithStdin(t, home, stdin, "opencode", "handle", "--json")
	require.NoError(t, err)
	assert.Contains(t, stdout, `"action":"fallback"`)
	assert.Contains(t, stdout, `"retry_safe":false`)
	assert.NotContains(t, stdout, `"auth":`)
	assert.Contains(t, stdout, `refresh oauth tokens`)
}

func TestOpencodeHandleReturnsFallbackJSONOnBadStdin(t *testing.T) {
	home := t.TempDir()
	require.NoError(t, writeAccountsFixture(home))

	stdout, _, err := executeCLIWithStdin(t, home, `{`, "opencode", "handle", "--json")
	require.NoError(t, err)
	assert.Contains(t, stdout, `"action":"fallback"`)
	assert.Contains(t, stdout, `"retry_safe":false`)
	assert.Contains(t, stdout, `decode opencode failure request`)
}

func TestOpencodeHandleReturnsFallbackJSONOnLookupFailure(t *testing.T) {
	home := t.TempDir()
	require.NoError(t, writeAccountsFixture(home))

	stdin := `{"provider":"openai","status":401,"message":"token expired","account_id":"missing"}`
	stdout, _, err := executeCLIWithStdin(t, home, stdin, "opencode", "handle", "--json")
	require.NoError(t, err)
	assert.Contains(t, stdout, `"action":"fallback"`)
	assert.Contains(t, stdout, `"retry_safe":false`)
	assert.Contains(t, stdout, `account not found`)
}

func TestOpencodeHandleReturnsFallbackJSONWhenAccountIDMissing(t *testing.T) {
	home := t.TempDir()
	require.NoError(t, writeAccountsFixture(home))

	stdin := `{"provider":"openai","status":401,"message":"token expired","account_id":""}`
	stdout, _, err := executeCLIWithStdin(t, home, stdin, "opencode", "handle", "--json")
	require.NoError(t, err)
	assert.Contains(t, stdout, `"action":"fallback"`)
	assert.Contains(t, stdout, `"retry_safe":false`)
	assert.Contains(t, stdout, `missing account_id; will not retry without current account context`)
	assert.NotContains(t, stdout, `"auth":`)
}

func TestOpencodeInstallWritesPluginAndConfig(t *testing.T) {
	home := t.TempDir()

	stdout, _, err := executeCLI(t, home, "opencode", "install")
	require.NoError(t, err)

	pluginPath := filepath.Join(home, ".config", "opencode", "plugins", "oa-plugin.js")
	assert.Contains(t, stdout, pluginPath)
	data, readErr := os.ReadFile(pluginPath)
	require.NoError(t, readErr)
	plugin := string(data)
	assert.Contains(t, plugin, `import { tool } from "@opencode-ai/plugin"`)
	assert.Contains(t, plugin, `export const OAPlugin = async ({ client, $ }) => {`)
	assert.Contains(t, plugin, `tool: {`)
	assert.Contains(t, plugin, `"oa-sync": tool({`)
	assert.Contains(t, plugin, `description: "Sync OpenCode auth with oa opencode sync"`)
	assert.Contains(t, plugin, "oa opencode sync")
	assert.Contains(t, plugin, `await client.tui.showToast({ body: { message, variant: "info" } })`)
	assert.Contains(t, plugin, `await client.tui.showToast({ body: { message, variant: "error" } })`)

	configPath := filepath.Join(home, ".config", "opencode", "package.json")
	configData, readConfigErr := os.ReadFile(configPath)
	require.NoError(t, readConfigErr)
	assert.JSONEq(t, `{"dependencies":{"@opencode-ai/plugin":"*"}}`, string(configData))
}

func TestOpencodeInstallMergesPluginDependencyIntoExistingConfigPackage(t *testing.T) {
	home := t.TempDir()
	configDir := filepath.Join(home, ".config", "opencode")
	require.NoError(t, os.MkdirAll(configDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(configDir, "package.json"), []byte(`{"name":"local-opencode-config","dependencies":{"left-pad":"1.3.0"}}`), 0o644))

	_, _, err := executeCLI(t, home, "opencode", "install")
	require.NoError(t, err)

	configData, readErr := os.ReadFile(filepath.Join(configDir, "package.json"))
	require.NoError(t, readErr)
	assert.JSONEq(t, `{"name":"local-opencode-config","dependencies":{"left-pad":"1.3.0","@opencode-ai/plugin":"*"}}`, string(configData))
}

func TestOpencodeDoctorReportsHealthyInstall(t *testing.T) {
	home := t.TempDir()
	require.NoError(t, writeAccountsFixture(home))
	binDir := writeFakeOABinary(t)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	_, _, err := executeCLI(t, home, "opencode", "install")
	require.NoError(t, err)

	stdout, _, err := executeCLI(t, home, "opencode", "doctor")
	require.NoError(t, err)
	assert.Contains(t, stdout, "plugin: ok")
	assert.Contains(t, stdout, "oa binary: ok")
	assert.Contains(t, stdout, "auth file: missing")
	assert.Contains(t, stdout, "account repo: ok accounts: 1")
}

func TestOpencodeDoctorReportsMissingOABinaryWhenNotOnPATH(t *testing.T) {
	home := t.TempDir()
	require.NoError(t, writeAccountsFixture(home))

	t.Setenv("PATH", "")

	stdout, _, err := executeCLI(t, home, "opencode", "doctor")
	require.NoError(t, err)
	assert.Contains(t, stdout, "oa binary: error not reachable")
}

func TestOpencodeDoctorReportsMissingPlugin(t *testing.T) {
	home := t.TempDir()
	require.NoError(t, writeAccountsFixture(home))

	stdout, _, err := executeCLI(t, home, "opencode", "doctor")
	require.NoError(t, err)
	assert.Contains(t, stdout, "plugin: missing")
}

func TestOpencodeSyncSelectsBestEligibleAccountAndWritesOpenAIAuthEntry(t *testing.T) {
	home := t.TempDir()
	require.NoError(t, writeOpencodeSyncAccountsFixture(home))
	accountID := "chatgpt-account-3"

	_, _, err := executeCLI(t, home,
		"auth", "set",
		"--account", "acc-2",
		"--method", "chatgpt",
		"--secret-key", "openai://acc-2/oauth_tokens",
		"--secret-value", fmt.Sprintf(`{"access_token":"access-token-222","refresh_token":"refresh-token-222","id_token":%q,"expires_at":1890000000}`, fakeJWT(`{"chatgpt_account_id":"chatgpt-account-2"}`)),
	)
	require.NoError(t, err)
	_, _, err = executeCLI(t, home,
		"auth", "set",
		"--account", "acc-3",
		"--method", "chatgpt",
		"--secret-key", "openai://acc-3/oauth_tokens",
		"--secret-value", fmt.Sprintf(`{"access_token":"access-token-333","refresh_token":"refresh-token-333","id_token":%q,"expires_at":1890000000}`, fakeJWT(fmt.Sprintf(`{"chatgpt_account_id":%q}`, accountID))),
	)
	require.NoError(t, err)

	stdout, _, err := executeCLI(t, home, "opencode", "sync")
	require.NoError(t, err)
	assert.Contains(t, stdout, "acc-3")

	authPath := filepath.Join(home, ".local", "share", "opencode", "auth.json")
	data, readErr := os.ReadFile(authPath)
	require.NoError(t, readErr)
	assert.Contains(t, string(data), `"openai"`)
	assert.Contains(t, string(data), fmt.Sprintf(`"accountId": "%s"`, accountID))
	assert.NotContains(t, string(data), `"accountId":"acc-1"`)
	assert.NotContains(t, string(data), `"codex"`)
}

func TestOpencodeSyncPreservesOtherProviderEntries(t *testing.T) {
	home := t.TempDir()
	require.NoError(t, writeOpencodeSyncAccountsFixture(home))

	_, _, err := executeCLI(t, home,
		"auth", "set",
		"--account", "acc-3",
		"--method", "chatgpt",
		"--secret-key", "openai://acc-3/oauth_tokens",
		"--secret-value", fmt.Sprintf(`{"access_token":"access-token-333","refresh_token":"refresh-token-333","id_token":%q,"expires_at":1890000000}`, fakeJWT(`{"chatgpt_account_id":"chatgpt-account-3"}`)),
	)
	require.NoError(t, err)

	authPath := filepath.Join(home, ".local", "share", "opencode", "auth.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(authPath), 0o755))
	require.NoError(t, os.WriteFile(authPath, []byte(`{
	  "anthropic": {
	    "type": "api_key",
	    "key": "anthropic-key"
	  },
	  "openai": {
	    "type": "oauth",
	    "access": "stale",
	    "refresh": "stale"
	  }
	}`), 0o600))

	_, _, err = executeCLI(t, home, "opencode", "sync")
	require.NoError(t, err)

	data, readErr := os.ReadFile(authPath)
	require.NoError(t, readErr)
	assert.Contains(t, string(data), `"anthropic"`)
	assert.Contains(t, string(data), `"key": "anthropic-key"`)
	assert.Contains(t, string(data), `"openai"`)
	assert.Contains(t, string(data), `"accountId": "chatgpt-account-3"`)
	assert.NotContains(t, string(data), `"access": "stale"`)
}

func TestOpencodeInstallSystemdWritesUserUnits(t *testing.T) {
	home := t.TempDir()

	stdout, _, err := executeCLI(t, home, "opencode", "install-systemd")
	require.NoError(t, err)
	assert.Contains(t, stdout, "oa-opencode-sync.service")
	assert.Contains(t, stdout, "oa-opencode-sync.timer")

	servicePath := filepath.Join(home, ".config", "systemd", "user", "oa-opencode-sync.service")
	timerPath := filepath.Join(home, ".config", "systemd", "user", "oa-opencode-sync.timer")
	serviceData, readServiceErr := os.ReadFile(servicePath)
	require.NoError(t, readServiceErr)
	timerData, readTimerErr := os.ReadFile(timerPath)
	require.NoError(t, readTimerErr)
	assert.Contains(t, string(serviceData), "ExecStart=")
	assert.Contains(t, string(serviceData), " opencode sync")
	assert.Contains(t, string(timerData), "OnUnitActiveSec=10m")
	assert.Contains(t, string(timerData), "WantedBy=timers.target")
}

func TestRenderOpencodeSystemdServiceQuotesExecutablePath(t *testing.T) {
	service := renderOpencodeSystemdService("/tmp/My App/oa")
	assert.Contains(t, service, `ExecStart="/tmp/My App/oa" opencode sync`)
}

func TestOpencodeSyncFallsBackWhenTopRankedAccountTokensAreInvalid(t *testing.T) {
	home := t.TempDir()
	require.NoError(t, writeOpencodeSyncAccountsFixture(home))

	_, _, err := executeCLI(t, home,
		"auth", "set",
		"--account", "acc-2",
		"--method", "chatgpt",
		"--secret-key", "openai://acc-2/oauth_tokens",
		"--secret-value", `not-json`,
	)
	require.NoError(t, err)
	_, _, err = executeCLI(t, home,
		"auth", "set",
		"--account", "acc-3",
		"--method", "chatgpt",
		"--secret-key", "openai://acc-3/oauth_tokens",
		"--secret-value", fmt.Sprintf(`{"access_token":"access-token-333","refresh_token":"refresh-token-333","id_token":%q,"expires_at":1890000000}`, fakeJWT(`{"chatgpt_account_id":"chatgpt-account-3"}`)),
	)
	require.NoError(t, err)

	stdout, _, err := executeCLI(t, home, "opencode", "sync")
	require.NoError(t, err)
	assert.Contains(t, stdout, "acc-3")

	authPath := filepath.Join(home, ".local", "share", "opencode", "auth.json")
	data, readErr := os.ReadFile(authPath)
	require.NoError(t, readErr)
	assert.Contains(t, string(data), `"accountId": "chatgpt-account-3"`)
}

func TestOpencodeSyncFallsBackWhenTopRankedAccountTokensAreIncomplete(t *testing.T) {
	home := t.TempDir()
	require.NoError(t, writeOpencodeWriteFailureAccountsFixture(home))

	_, _, err := executeCLI(t, home,
		"auth", "set",
		"--account", "acc-2",
		"--method", "chatgpt",
		"--secret-key", "openai://acc-2/oauth_tokens",
		"--secret-value", `{"access_token":"access-token-222","refresh_token":"","id_token":"","expires_at":1890000000}`,
	)
	require.NoError(t, err)
	_, _, err = executeCLI(t, home,
		"auth", "set",
		"--account", "acc-3",
		"--method", "chatgpt",
		"--secret-key", "openai://acc-3/oauth_tokens",
		"--secret-value", fmt.Sprintf(`{"access_token":"access-token-333","refresh_token":"refresh-token-333","id_token":%q,"expires_at":1890000000}`, fakeJWT(`{"chatgpt_account_id":"chatgpt-account-3"}`)),
	)
	require.NoError(t, err)

	stdout, _, err := executeCLI(t, home, "opencode", "sync")
	require.NoError(t, err)
	assert.Contains(t, stdout, "acc-3")

	authPath := filepath.Join(home, ".local", "share", "opencode", "auth.json")
	data, readErr := os.ReadFile(authPath)
	require.NoError(t, readErr)
	assert.Contains(t, string(data), `"accountId": "chatgpt-account-3"`)
}

func TestOpencodeSyncDoesNotFallThroughOnAuthFileWriteFailures(t *testing.T) {
	home := t.TempDir()
	require.NoError(t, writeOpencodeWriteFailureAccountsFixture(home))

	blockedDir := filepath.Join(home, ".local", "share", "opencode")
	require.NoError(t, os.MkdirAll(filepath.Dir(blockedDir), 0o755))
	require.NoError(t, os.WriteFile(blockedDir, []byte("blocked"), 0o644))

	_, _, err := executeCLI(t, home,
		"auth", "set",
		"--account", "acc-2",
		"--method", "chatgpt",
		"--secret-key", "openai://acc-2/oauth_tokens",
		"--secret-value", fmt.Sprintf(`{"access_token":"access-token-222","refresh_token":"refresh-token-222","id_token":%q,"expires_at":1890000000}`, fakeJWT(`{"chatgpt_account_id":"chatgpt-account-2"}`)),
	)
	require.NoError(t, err)
	_, _, err = executeCLI(t, home,
		"auth", "set",
		"--account", "acc-3",
		"--method", "chatgpt",
		"--secret-key", "openai://acc-3/oauth_tokens",
		"--secret-value", `not-json`,
	)
	require.NoError(t, err)

	_, _, err = executeCLI(t, home, "opencode", "sync")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "opencode auth file")
	assert.NotContains(t, err.Error(), "decode tokens")
}

func TestOpencodeSyncFailsOnMalformedExistingAuthFile(t *testing.T) {
	home := t.TempDir()
	require.NoError(t, writeOpencodeSyncAccountsFixture(home))

	_, _, err := executeCLI(t, home,
		"auth", "set",
		"--account", "acc-3",
		"--method", "chatgpt",
		"--secret-key", "openai://acc-3/oauth_tokens",
		"--secret-value", fmt.Sprintf(`{"access_token":"access-token-333","refresh_token":"refresh-token-333","id_token":%q,"expires_at":1890000000}`, fakeJWT(`{"chatgpt_account_id":"chatgpt-account-3"}`)),
	)
	require.NoError(t, err)

	authPath := filepath.Join(home, ".local", "share", "opencode", "auth.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(authPath), 0o755))
	require.NoError(t, os.WriteFile(authPath, []byte(`{"openai":`), 0o600))

	_, _, err = executeCLI(t, home, "opencode", "sync")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "decode opencode auth file")

	data, readErr := os.ReadFile(authPath)
	require.NoError(t, readErr)
	assert.Equal(t, `{"openai":`, string(data))
}

func TestOpencodeSyncPreservesExistingAuthFileMode(t *testing.T) {
	home := t.TempDir()
	require.NoError(t, writeOpencodeSyncAccountsFixture(home))

	_, _, err := executeCLI(t, home,
		"auth", "set",
		"--account", "acc-3",
		"--method", "chatgpt",
		"--secret-key", "openai://acc-3/oauth_tokens",
		"--secret-value", fmt.Sprintf(`{"access_token":"access-token-333","refresh_token":"refresh-token-333","id_token":%q,"expires_at":1890000000}`, fakeJWT(`{"chatgpt_account_id":"chatgpt-account-3"}`)),
	)
	require.NoError(t, err)

	authPath := filepath.Join(home, ".local", "share", "opencode", "auth.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(authPath), 0o755))
	require.NoError(t, os.WriteFile(authPath, []byte(`{"anthropic":{"type":"api_key","key":"anthropic-key"}}`), 0o640))

	_, _, err = executeCLI(t, home, "opencode", "sync")
	require.NoError(t, err)

	info, statErr := os.Stat(authPath)
	require.NoError(t, statErr)
	assert.Equal(t, os.FileMode(0o640), info.Mode().Perm())
}

func TestOpencodeSyncRejectsTokensWithoutExpiry(t *testing.T) {
	home := t.TempDir()
	require.NoError(t, writeOpencodeSingleSyncAccountFixture(home, "acc-2", "Only Choice"))

	_, _, err := executeCLI(t, home,
		"auth", "set",
		"--account", "acc-2",
		"--method", "chatgpt",
		"--secret-key", "openai://acc-2/oauth_tokens",
		"--secret-value", fmt.Sprintf(`{"access_token":"access-token-222","refresh_token":"refresh-token-222","id_token":%q}`, fakeJWT(`{"chatgpt_account_id":"chatgpt-account-2"}`)),
	)
	require.NoError(t, err)

	_, _, err = executeCLI(t, home, "opencode", "sync")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing token expiry")

	_, statErr := os.Stat(filepath.Join(home, ".local", "share", "opencode", "auth.json"))
	assert.ErrorIs(t, statErr, os.ErrNotExist)
}

func TestRotateOpencodeCommandIsRemoved(t *testing.T) {
	home := t.TempDir()
	require.NoError(t, writeAccountsFixture(home))

	_, _, err := executeCLI(t, home, "rotate", "opencode")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown command \"rotate\"")
}

func TestAccountListShowsConfiguredAccounts(t *testing.T) {
	home := t.TempDir()
	require.NoError(t, writeAccountsFixture(home))

	stdout, _, err := executeCLI(t, home, "account", "list")
	require.NoError(t, err)
	assert.Contains(t, stdout, "acc-1")
	assert.Contains(t, stdout, "Primary")
}

func TestUsageSetSubcommandIsRemoved(t *testing.T) {
	home := t.TempDir()
	require.NoError(t, writeAccountsFixture(home))

	_, _, err := executeCLI(t, home, "usage", "set", "--account", "acc-1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown command \"set\"")
}

func TestUsageCommandFetchesLimitsAndRendersStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/wham/usage":
			assert.Equal(t, "Bearer access-token-123", r.Header.Get("Authorization"))
			_, _ = fmt.Fprint(w, `{"plan_type":"pro","rate_limit":{"allowed":true,"limit_reached":false,"primary_window":{"used_percent":21,"limit_window_seconds":18000,"reset_after_seconds":120,"reset_at":1893456000},"secondary_window":{"used_percent":47,"limit_window_seconds":604800,"reset_after_seconds":3600,"reset_at":1893888000}}}`)
		case r.URL.Path == "/subscriptions":
			w.WriteHeader(http.StatusNotFound)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	t.Setenv("OA_USAGE_BASE_URL", server.URL)

	home := t.TempDir()
	require.NoError(t, writeAccountsFixture(home))

	_, _, err := executeCLI(t, home,
		"auth", "set",
		"--account", "acc-1",
		"--method", "chatgpt",
		"--secret-key", "openai://acc-1/oauth_tokens",
		"--secret-value", `{"access_token":"access-token-123","id_token":""}`,
	)
	require.NoError(t, err)

	stdout, _, err := executeCLI(t, home, "usage", "--account", "acc-1")
	require.NoError(t, err)
	assert.Contains(t, stdout, "5hours limit:")
	assert.Contains(t, stdout, "weekly limit:")
	assert.Contains(t, stdout, "79% left")
	assert.Contains(t, stdout, "53% left")
}

func TestStatusAliasFetchesLimitsAndRendersStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/wham/usage":
			assert.Equal(t, "Bearer access-token-123", r.Header.Get("Authorization"))
			_, _ = fmt.Fprint(w, `{"plan_type":"pro","rate_limit":{"allowed":true,"limit_reached":false,"primary_window":{"used_percent":21,"limit_window_seconds":18000,"reset_after_seconds":120,"reset_at":1893456000},"secondary_window":{"used_percent":47,"limit_window_seconds":604800,"reset_after_seconds":3600,"reset_at":1893888000}}}`)
		case r.URL.Path == "/subscriptions":
			w.WriteHeader(http.StatusNotFound)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	t.Setenv("OA_USAGE_BASE_URL", server.URL)

	home := t.TempDir()
	require.NoError(t, writeAccountsFixture(home))

	_, _, err := executeCLI(t, home,
		"auth", "set",
		"--account", "acc-1",
		"--method", "chatgpt",
		"--secret-key", "openai://acc-1/oauth_tokens",
		"--secret-value", `{"access_token":"access-token-123","id_token":""}`,
	)
	require.NoError(t, err)

	stdout, _, err := executeCLI(t, home, "status", "--account", "acc-1")
	require.NoError(t, err)
	assert.Contains(t, stdout, "5hours limit:")
	assert.Contains(t, stdout, "weekly limit:")
	assert.Contains(t, stdout, "79% left")
	assert.Contains(t, stdout, "53% left")
}

func TestUsageCommandJSONOutput(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `{"plan_type":"pro","rate_limit":{"allowed":true,"limit_reached":false,"primary_window":{"used_percent":21,"limit_window_seconds":18000,"reset_after_seconds":120,"reset_at":1893456000},"secondary_window":{"used_percent":47,"limit_window_seconds":604800,"reset_after_seconds":3600,"reset_at":1893888000}}}`)
	}))
	defer server.Close()

	t.Setenv("OA_USAGE_BASE_URL", server.URL)

	home := t.TempDir()
	require.NoError(t, writeAccountsFixture(home))

	_, _, err := executeCLI(t, home,
		"auth", "set",
		"--account", "acc-1",
		"--method", "chatgpt",
		"--secret-key", "openai://acc-1/oauth_tokens",
		"--secret-value", `{"access_token":"access-token-123","id_token":""}`,
	)
	require.NoError(t, err)

	stdout, _, err := executeCLI(t, home, "usage", "--account", "acc-1", "--json")
	require.NoError(t, err)
	assert.True(t, json.Valid([]byte(stdout)))
	assert.Contains(t, stdout, "\"DailyLimit\"")
	assert.Contains(t, stdout, "\"WeeklyLimit\"")
}

func TestUsageCommandShowsFetchingSpinnerMessage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		_, _ = fmt.Fprint(w, `{"plan_type":"pro","rate_limit":{"allowed":true,"limit_reached":false,"primary_window":{"used_percent":21,"limit_window_seconds":18000,"reset_after_seconds":120,"reset_at":1893456000},"secondary_window":{"used_percent":47,"limit_window_seconds":604800,"reset_after_seconds":3600,"reset_at":1893888000}}}`)
	}))
	defer server.Close()

	t.Setenv("OA_USAGE_BASE_URL", server.URL)

	home := t.TempDir()
	require.NoError(t, writeAccountsFixture(home))

	_, _, err := executeCLI(t, home,
		"auth", "set",
		"--account", "acc-1",
		"--method", "chatgpt",
		"--secret-key", "openai://acc-1/oauth_tokens",
		"--secret-value", `{"access_token":"access-token-123","id_token":""}`,
	)
	require.NoError(t, err)

	_, stderr, err := executeCLI(t, home, "usage", "--account", "acc-1")
	require.NoError(t, err)
	assert.Contains(t, stderr, "Fetching usage limits")
}

func TestUsageCommandReturnsFetchError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = fmt.Fprint(w, `{"error":"invalid_token"}`)
	}))
	defer server.Close()

	t.Setenv("OA_USAGE_BASE_URL", server.URL)

	home := t.TempDir()
	require.NoError(t, writeAccountsFixture(home))

	_, _, err := executeCLI(t, home,
		"auth", "set",
		"--account", "acc-1",
		"--method", "chatgpt",
		"--secret-key", "openai://acc-1/oauth_tokens",
		"--secret-value", `{"access_token":"bad-token","id_token":""}`,
	)
	require.NoError(t, err)

	_, _, err = executeCLI(t, home, "usage", "--account", "acc-1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "session expired")
	assert.Contains(t, err.Error(), "oa auth login browser --account acc-1")
}

func TestUsageCommandUpdatesAccountNameFromTokenEmail(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `{"plan_type":"team","rate_limit":{"allowed":true,"limit_reached":false,"primary_window":{"used_percent":30,"limit_window_seconds":18000,"reset_after_seconds":120,"reset_at":1893456000},"secondary_window":{"used_percent":10,"limit_window_seconds":604800,"reset_after_seconds":3600,"reset_at":1893888000}}}`)
	}))
	defer server.Close()

	t.Setenv("OA_USAGE_BASE_URL", server.URL)

	home := t.TempDir()
	require.NoError(t, writeAccountsFixture(home))

	idToken := fakeJWT(`{"email":"email@adress.com"}`)
	_, _, err := executeCLI(t, home,
		"auth", "set",
		"--account", "acc-1",
		"--method", "chatgpt",
		"--secret-key", "openai://acc-1/oauth_tokens",
		"--secret-value", fmt.Sprintf(`{"access_token":"ok-token","id_token":"%s"}`, idToken),
	)
	require.NoError(t, err)

	stdout, _, err := executeCLI(t, home, "usage", "--account", "acc-1")
	require.NoError(t, err)
	assert.Contains(t, stdout, "Account #acc-1: email@adress.com (Team)")
}

func TestUsageCommandRefreshesExpiredAccessTokenAndRetries(t *testing.T) {
	var oldTokenCalls int
	var newTokenCalls int
	var refreshCalls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/oauth/token":
			refreshCalls++
			require.Equal(t, http.MethodPost, r.Method)
			require.NoError(t, r.ParseForm())
			assert.Equal(t, "refresh_token", r.Form.Get("grant_type"))
			assert.Equal(t, "test-client-id", r.Form.Get("client_id"))
			assert.Equal(t, "refresh-token-123", r.Form.Get("refresh_token"))
			_, _ = fmt.Fprint(w, `{"access_token":"new-token","refresh_token":"refresh-token-456","id_token":"","token_type":"Bearer","expires_in":3600}`)
		case r.URL.Path == "/wham/usage":
			authz := r.Header.Get("Authorization")
			if authz == "Bearer old-token" {
				oldTokenCalls++
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = fmt.Fprint(w, `{"error":"invalid_token"}`)
				return
			}
			assert.Equal(t, "Bearer new-token", authz)
			newTokenCalls++
			_, _ = fmt.Fprint(w, `{"plan_type":"pro","rate_limit":{"primary_window":{"used_percent":21,"limit_window_seconds":18000,"reset_at":1893456000},"secondary_window":{"used_percent":47,"limit_window_seconds":604800,"reset_at":1893888000}}}`)
		case r.URL.Path == "/subscriptions":
			w.WriteHeader(http.StatusNotFound)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	t.Setenv("OA_USAGE_BASE_URL", server.URL)
	t.Setenv("OA_AUTH_ISSUER", server.URL)
	t.Setenv("OA_AUTH_CLIENT_ID", "test-client-id")

	home := t.TempDir()
	require.NoError(t, writeAccountsFixture(home))

	_, _, err := executeCLI(t, home,
		"auth", "set",
		"--account", "acc-1",
		"--method", "chatgpt",
		"--secret-key", "openai://acc-1/oauth_tokens",
		"--secret-value", `{"access_token":"old-token","refresh_token":"refresh-token-123","id_token":"","expires_at":1}`,
	)
	require.NoError(t, err)

	stdout, _, err := executeCLI(t, home, "usage", "--account", "acc-1")
	require.NoError(t, err)
	assert.LessOrEqual(t, oldTokenCalls, 1)
	assert.GreaterOrEqual(t, refreshCalls, 1)
	assert.GreaterOrEqual(t, newTokenCalls, 1)
	assert.Contains(t, stdout, "5hours limit:")
}

func TestUsageCommandExpiredErrorIncludesEmailAndType(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = fmt.Fprint(w, `{"error":"invalid_token"}`)
	}))
	defer server.Close()

	t.Setenv("OA_USAGE_BASE_URL", server.URL)

	home := t.TempDir()
	require.NoError(t, writeAccountsFixture(home))

	idToken := fakeJWT(`{"email":"email@adress.com"}`)
	_, _, err := executeCLI(t, home,
		"auth", "set",
		"--account", "acc-1",
		"--method", "chatgpt",
		"--secret-key", "openai://acc-1/oauth_tokens",
		"--secret-value", fmt.Sprintf(`{"access_token":"bad-token","id_token":"%s"}`, idToken),
	)
	require.NoError(t, err)

	_, stderr, err := executeCLI(t, home, "usage", "--account", "acc-1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "account acc-1 (email@adress.com, Unknown): session expired")
	assert.Contains(t, stderr, "account acc-1 (email@adress.com, Unknown): session expired")
}

func TestUsageCommandFetchesSubscriptionAndRendersRenewal(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/wham/usage":
			_, _ = fmt.Fprint(w, `{"plan_type":"plus","rate_limit":{"allowed":true,"limit_reached":false,"primary_window":{"used_percent":21,"limit_window_seconds":18000,"reset_after_seconds":120,"reset_at":1893456000},"secondary_window":{"used_percent":47,"limit_window_seconds":604800,"reset_after_seconds":3600,"reset_at":1893888000}}}`)
		case r.URL.Path == "/subscriptions":
			_, _ = fmt.Fprint(w, `{"plan_type":"plus","active_start":"2026-02-14T07:41:19Z","active_until":"2026-03-14T07:41:19Z","will_renew":true,"billing_period":"monthly","billing_currency":"EUR","is_delinquent":false}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	t.Setenv("OA_USAGE_BASE_URL", server.URL)

	home := t.TempDir()
	require.NoError(t, writeAccountsFixture(home))

	_, _, err := executeCLIWithHomeAndApp(t, home, func(app *app) {
		app.now = func() time.Time { return time.Date(2026, 2, 14, 11, 0, 0, 0, time.UTC) }
	},
		"auth", "set",
		"--account", "acc-1",
		"--method", "chatgpt",
		"--secret-key", "openai://acc-1/oauth_tokens",
		"--secret-value", `{"access_token":"access-token-123","id_token":""}`,
	)
	require.NoError(t, err)

	stdout, _, err := executeCLIWithHomeAndApp(t, home, func(app *app) {
		app.now = func() time.Time { return time.Date(2026, 2, 14, 11, 0, 0, 0, time.UTC) }
	}, "usage", "--account", "acc-1")
	require.NoError(t, err)
	assert.Contains(t, stdout, "renewal:")
	assert.Contains(t, stdout, "renews in")
	assert.Contains(t, stdout, "14 Mar")
}

func executeCLI(t *testing.T, home string, args ...string) (string, string, error) {
	t.Helper()
	t.Setenv("HOME", home)

	root := newRootCmd()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	root.SetOut(stdout)
	root.SetErr(stderr)
	root.SetArgs(args)

	executeErr := root.Execute()
	return stdout.String(), stderr.String(), executeErr
}

func executeCLIWithStdin(t *testing.T, home, stdin string, args ...string) (string, string, error) {
	t.Helper()
	t.Setenv("HOME", home)

	root := newRootCmd()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	root.SetIn(bytes.NewBufferString(stdin))
	root.SetOut(stdout)
	root.SetErr(stderr)
	root.SetArgs(args)

	executeErr := root.Execute()
	return stdout.String(), stderr.String(), executeErr
}

func executeCLIWithHomeAndApp(t *testing.T, home string, configure func(*app), args ...string) (string, string, error) {
	t.Helper()
	t.Setenv("HOME", home)

	app, err := wireApp()
	require.NoError(t, err)
	if configure != nil {
		configure(app)
	}

	root := newRootCmdWithApp(app)
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	root.SetOut(stdout)
	root.SetErr(stderr)
	root.SetArgs(args)

	executeErr := root.Execute()
	return stdout.String(), stderr.String(), executeErr
}

func writeAccountsFixture(home string) error {
	configDir := filepath.Join(home, ".codex")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		return err
	}

	accounts := `version = 1

[[accounts]]
id = "acc-1"
name = "Primary"

[accounts.metadata]
provider = "openai"
model = "gpt-5"

[accounts.auth]
method = ""
secret_ref = ""
`

	return os.WriteFile(filepath.Join(configDir, "accounts.toml"), []byte(accounts), 0o644)
}

func writeFakeOABinary(t *testing.T) string {
	t.Helper()

	binDir := t.TempDir()
	path := filepath.Join(binDir, "oa")
	contents := []byte("#!/bin/sh\nexit 0\n")
	require.NoError(t, os.WriteFile(path, contents, 0o755))

	return binDir
}

func writeAccountsFixtureWithChatGPTAuth(home string) error {
	configDir := filepath.Join(home, ".codex")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		return err
	}

	accounts := `version = 1

[[accounts]]
id = "acc-1"
name = "Primary"

[accounts.metadata]
provider = "openai"
model = "gpt-5"
secret_ref = "openai://acc-1/oauth_tokens"

[accounts.auth]
method = "chatgpt"
secret_ref = "openai://acc-1/oauth_tokens"
`

	return os.WriteFile(filepath.Join(configDir, "accounts.toml"), []byte(accounts), 0o644)
}

func writeOpencodeSyncAccountsFixture(home string) error {
	configDir := filepath.Join(home, ".codex")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		return err
	}

	accounts := `version = 1

[[accounts]]
id = "acc-1"
name = "Cooling Down"

[accounts.metadata]
provider = "openai"
model = "gpt-5"

[accounts.auth]
method = ""
secret_ref = ""

[accounts.limits.daily]
percent = 100
resets_at = "2026-04-05T13:00:00Z"
captured_at = "2026-04-05T12:00:00Z"

[[accounts]]
id = "acc-2"
name = "Okay"

[accounts.metadata]
provider = "openai"
model = "gpt-5"

[accounts.auth]
method = "chatgpt"
secret_ref = "openai://acc-2/oauth_tokens"

[accounts.limits.daily]
percent = 40
resets_at = "2026-04-05T13:00:00Z"
captured_at = "2026-04-05T12:00:00Z"

[accounts.limits.weekly]
percent = 35
resets_at = "2026-04-07T12:00:00Z"
captured_at = "2026-04-05T12:00:00Z"

[accounts.subscription]
active_start = "2026-04-01T00:00:00Z"
active_until = "2026-05-01T00:00:00Z"
will_renew = true
billing_period = "monthly"
billing_currency = "USD"
is_delinquent = false
captured_at = "2026-04-05T12:00:00Z"

[[accounts]]
id = "acc-3"
name = "Best"

[accounts.metadata]
provider = "openai"
model = "gpt-5"

[accounts.auth]
method = "chatgpt"
secret_ref = "openai://acc-3/oauth_tokens"

[accounts.limits.daily]
percent = 15
resets_at = "2026-04-05T13:00:00Z"
captured_at = "2026-04-05T12:00:00Z"

[accounts.limits.weekly]
percent = 25
resets_at = "2026-04-07T12:00:00Z"
captured_at = "2026-04-05T12:00:00Z"

[accounts.subscription]
active_start = "2026-04-01T00:00:00Z"
active_until = "2026-05-01T00:00:00Z"
will_renew = true
billing_period = "monthly"
billing_currency = "USD"
is_delinquent = false
captured_at = "2026-04-05T12:00:00Z"
`

	return os.WriteFile(filepath.Join(configDir, "accounts.toml"), []byte(accounts), 0o644)
}

func writeOpencodeWriteFailureAccountsFixture(home string) error {
	configDir := filepath.Join(home, ".codex")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		return err
	}

	accounts := `version = 1

[[accounts]]
id = "acc-2"
name = "Top Choice"

[accounts.metadata]
provider = "openai"
model = "gpt-5"

[accounts.auth]
method = "chatgpt"
secret_ref = "openai://acc-2/oauth_tokens"

[accounts.limits.daily]
percent = 5
resets_at = "2026-04-05T13:00:00Z"
captured_at = "2026-04-05T12:00:00Z"

[accounts.limits.weekly]
percent = 10
resets_at = "2026-04-07T12:00:00Z"
captured_at = "2026-04-05T12:00:00Z"

[accounts.subscription]
active_start = "2026-04-01T00:00:00Z"
active_until = "2026-05-01T00:00:00Z"
will_renew = true
billing_period = "monthly"
billing_currency = "USD"
is_delinquent = false
captured_at = "2026-04-05T12:00:00Z"

[[accounts]]
id = "acc-3"
name = "Backup"

[accounts.metadata]
provider = "openai"
model = "gpt-5"

[accounts.auth]
method = "chatgpt"
secret_ref = "openai://acc-3/oauth_tokens"

[accounts.limits.daily]
percent = 20
resets_at = "2026-04-05T13:00:00Z"
captured_at = "2026-04-05T12:00:00Z"

[accounts.limits.weekly]
percent = 30
resets_at = "2026-04-07T12:00:00Z"
captured_at = "2026-04-05T12:00:00Z"

[accounts.subscription]
active_start = "2026-04-01T00:00:00Z"
active_until = "2026-05-01T00:00:00Z"
will_renew = true
billing_period = "monthly"
billing_currency = "USD"
is_delinquent = false
captured_at = "2026-04-05T12:00:00Z"
`

	return os.WriteFile(filepath.Join(configDir, "accounts.toml"), []byte(accounts), 0o644)
}

func writeOpencodeSingleSyncAccountFixture(home, accountID, accountName string) error {
	configDir := filepath.Join(home, ".codex")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		return err
	}

	accounts := fmt.Sprintf(`version = 1

[[accounts]]
id = %q
name = %q

[accounts.metadata]
provider = "openai"
model = "gpt-5"

[accounts.auth]
method = "chatgpt"
secret_ref = %q

[accounts.limits.daily]
percent = 5
resets_at = "2026-04-05T13:00:00Z"
captured_at = "2026-04-05T12:00:00Z"

[accounts.limits.weekly]
percent = 10
resets_at = "2026-04-07T12:00:00Z"
captured_at = "2026-04-05T12:00:00Z"

[accounts.subscription]
active_start = "2026-04-01T00:00:00Z"
active_until = "2026-05-01T00:00:00Z"
will_renew = true
billing_period = "monthly"
billing_currency = "USD"
is_delinquent = false
captured_at = "2026-04-05T12:00:00Z"
`, accountID, accountName, fmt.Sprintf("openai://%s/oauth_tokens", accountID))

	return os.WriteFile(filepath.Join(configDir, "accounts.toml"), []byte(accounts), 0o644)
}

func fakeJWT(payload string) string {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none","typ":"JWT"}`))
	body := base64.RawURLEncoding.EncodeToString([]byte(payload))
	return header + "." + body + ".sig"
}

func TestAuthCheckFailsForExpiredSession(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = fmt.Fprint(w, `{"error":"invalid_token"}`)
	}))
	defer server.Close()

	t.Setenv("OA_USAGE_BASE_URL", server.URL)

	home := t.TempDir()
	require.NoError(t, writeAccountsFixture(home))

	_, _, err := executeCLI(t, home,
		"auth", "set",
		"--account", "acc-1",
		"--method", "chatgpt",
		"--secret-key", "openai://acc-1/oauth_tokens",
		"--secret-value", `{"access_token":"bad-token","refresh_token":"","id_token":""}`,
	)
	require.NoError(t, err)

	_, stderr, err := executeCLI(t, home, "auth", "check", "--account", "acc-1")
	require.Error(t, err)
	assert.Contains(t, err.Error()+stderr, "session expired")
}

func TestAuthCheckReportsOKAfterSuccessfulFetch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/wham/usage" {
			_, _ = fmt.Fprint(w, `{"plan_type":"pro","rate_limit":{"primary_window":{"used_percent":20,"limit_window_seconds":18000,"reset_at":1893456000},"secondary_window":{"used_percent":50,"limit_window_seconds":604800,"reset_at":1893888000}}}`)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	t.Setenv("OA_USAGE_BASE_URL", server.URL)

	home := t.TempDir()
	require.NoError(t, writeAccountsFixture(home))

	_, _, err := executeCLI(t, home,
		"auth", "set",
		"--account", "acc-1",
		"--method", "chatgpt",
		"--secret-key", "openai://acc-1/oauth_tokens",
		"--secret-value", `{"access_token":"valid-token","id_token":""}`,
	)
	require.NoError(t, err)

	stdout, _, err := executeCLI(t, home, "auth", "check", "--account", "acc-1")
	require.NoError(t, err)
	assert.Contains(t, stdout, "ok")
	assert.Contains(t, stdout, "acc-1")
}

func TestAuthCheckSkipsNonChatGPTAccount(t *testing.T) {
	home := t.TempDir()
	require.NoError(t, writeAccountsFixture(home))

	_, _, err := executeCLI(t, home,
		"auth", "set",
		"--account", "acc-1",
		"--method", "api_key",
		"--secret-key", "openai://acc-1/api_key",
		"--secret-value", "sk-test-key",
	)
	require.NoError(t, err)

	stdout, _, err := executeCLI(t, home, "auth", "check", "--account", "acc-1")
	require.NoError(t, err)
	assert.Contains(t, stdout, "no ChatGPT accounts to check")
}
