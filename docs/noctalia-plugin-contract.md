# Noctalia plugin contract

## First release scope

The first Noctalia plugin release should expose:

- a panel with the current account snapshot
- the current recommended account for sync
- one-click sync actions for `opencode`, `codex`, `pi`, and `all`
- a small bar widget that opens the panel and shows recommendation state
- IPC actions for `togglePanel`, `refresh`, and `sync`

Launcher-provider search can wait until the panel workflow feels solid.

## Existing CLI commands safe for direct plugin use

These commands already have stable enough success/error envelopes for the plugin:

- `oa sync opencode --json`
- `oa sync codex --json`
- `oa sync pi --json`
- `oa sync --all --json`
- `oa version --json`

These commands are useful for humans, but not ideal as the main plugin contract:

- `oa usage --json` / `oa status --json` — returns raw Go status structs
- `oa auth check --json` — diagnostic-oriented, not a panel snapshot
- `oa account list --json` — too small on its own for the plugin UI

## Gap closed for Noctalia

Use the dedicated snapshot command instead of raw status JSON:

- `oa noctalia snapshot --json`
- `oa noctalia snapshot --json --refresh`

This command keeps recommendation and eligibility logic in Go and returns warnings in stdout JSON, so QML only renders and triggers actions.

## Snapshot DTO v1

```json
{
  "schema_version": 1,
  "generated_at": "2026-05-23T12:10:00Z",
  "refreshed": true,
  "refresh_command": ["noctalia", "snapshot", "--json", "--refresh"],
  "recommendation": {
    "available": true,
    "account_id": "acc-3",
    "account_name": "Best",
    "rank": 1,
    "message": "recommended account: Best"
  },
  "accounts": [
    {
      "id": "acc-3",
      "name": "Best",
      "provider": "openai",
      "model": "gpt-5",
      "plan_type": "pro",
      "auth_method": "chatgpt",
      "auth_configured": true,
      "recommendation": {
        "rank": 1,
        "selected": true,
        "eligible": true
      },
      "daily": {
        "percent_used": 21,
        "percent_remaining": 79,
        "resets_at": "2026-05-23T17:00:00Z",
        "captured_at": "2026-05-23T12:10:00Z"
      },
      "weekly": {
        "percent_used": 47,
        "percent_remaining": 53,
        "resets_at": "2026-05-30T17:00:00Z",
        "captured_at": "2026-05-23T12:10:00Z"
      },
      "subscription": {
        "active_start": "2026-05-01T00:00:00Z",
        "active_until": "2026-06-01T00:00:00Z",
        "will_renew": true,
        "billing_period": "monthly",
        "billing_currency": "USD",
        "is_delinquent": false,
        "captured_at": "2026-05-23T12:10:00Z"
      }
    }
  ],
  "sync_targets": [
    { "id": "opencode", "label": "OpenCode", "command": ["sync", "opencode", "--json"] },
    { "id": "codex", "label": "Codex", "command": ["sync", "codex", "--json"] },
    { "id": "pi", "label": "Pi", "command": ["sync", "pi", "--json"] },
    { "id": "all", "label": "All", "command": ["sync", "--all", "--json"] }
  ],
  "warnings": [
    {
      "account_id": "acc-2",
      "message": "account acc-2: refresh oauth tokens after unauthorized usage response: ..."
    }
  ]
}
```

## Contract rules

- `schema_version` is mandatory and must change on breaking DTO changes.
- warnings belong in stdout JSON, not stderr, for snapshot reads.
- sync actions are discovered from `sync_targets[*].command`.
- QML may format labels, but must not reimplement recommendation or account-selection rules.
