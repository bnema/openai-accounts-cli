# openai-accounts-cli (oa)

**oa** manages OpenAI account auth, usage, and local tool sync from one CLI.

Use it to keep multiple OpenAI accounts in one place, inspect their ChatGPT limits, choose a usable account, and sync ChatGPT OAuth credentials into OpenCode, Codex, and Pi.

## What it does

- Stores account metadata and secret references in `~/.codex/accounts.toml`
- Stores secrets via `pass`, with file fallback at `~/.codex/secrets`
- Supports API key and ChatGPT OAuth auth
- Fetches 5-hour and weekly ChatGPT usage limits from OpenAI
- Recommends accounts by usage pressure before subscription end, weekly reset, and 5-hour reset
- Shows subscription renewal countdowns
- Syncs ChatGPT OAuth credentials into OpenCode, Codex, and Pi
- Groups local tool commands under stable top-level verbs: `install`, `handle`, and `sync`

## Install

```bash
go install github.com/bnema/openai-accounts-cli/cmd/oa@latest
# or from a clone
go install ./cmd/oa
```

## Commands

```text
oa
|- account                         Manage accounts
|  |- list                         List configured accounts
|  `- remove <id>                  Remove an account and its credentials
|- auth                            Manage account authentication
|  |- check [--account <id>]       Verify authentication for one or all ChatGPT accounts
|  |- login                        Start account login flows
|  |  |- browser [--account <id>]  Start browser login flow
|  |  `- device [--account <id>]   Start device login flow (not implemented)
|  |- remove --account <id>        Remove account authentication
|  `- set [--account <id>]         Set account authentication (`--method`, `--secret-key`, `--secret-value`)
|- handle                          Handle local tool integration callbacks
|  `- opencode --json              Handle OpenCode requests via JSON stdin/stdout
|- install                         Install local tool integrations
|  |- opencode                     Install OpenCode integration
|  `- pi                           Install Pi auth hot-reload extension
|- opencode                        Manage OpenCode integration
|  |- doctor                       Check OpenCode integration
|  `- install-systemd              Install a systemd user timer for OpenCode sync
|- sync [--all] [--evenly]         Sync ChatGPT OAuth auth into local tools
|  |- codex                        Sync Codex auth
|  |- opencode                     Sync OpenCode auth
|  `- pi                           Sync Pi auth (alias: `pi-mono`)
|- usage [--account <id>] [--json] Fetch and display account usage limits (alias: `status`)
`- version                         Print version information
```

## Command migration

Since v1.9, tool integration commands use stable top-level verbs. If you used the old OpenCode-only commands, switch to these forms:

| Old command | Current command |
|-------------|-----------------|
| `oa opencode install` | `oa install opencode` |
| `oa opencode handle --json` | `oa handle opencode --json` |
| `oa opencode sync` | `oa sync opencode` |

`oa opencode sync` still exists as a hidden deprecated alias. Use `oa sync opencode` in scripts.

## Tool integrations

### OpenCode

Install the OpenCode plugin:

```bash
oa install opencode
```

The plugin adds an `oa-sync` tool that runs `oa sync opencode` from OpenCode.

Handle OpenCode recovery requests from JSON stdin/stdout:

```bash
oa handle opencode --json
```

Sync the best eligible ChatGPT OAuth account into OpenCode:

```bash
oa sync opencode
```

Optional systemd timer:

```bash
oa opencode install-systemd
```

### Codex and Pi

Sync the best eligible ChatGPT OAuth account into Codex, Pi, or every supported target:

```bash
oa sync codex
oa sync pi
oa sync --all
```

Install the Pi extension once to let active Pi sessions pick up `oa sync pi` changes without restarting:

```bash
oa install pi
# then run /reload in already-open Pi sessions
```

The installed extension watches `~/.pi/agent/auth.json`, reloads Pi auth in memory before the next provider call, and tries `oa sync pi --evenly` after OpenAI Codex HTTP 401/429 responses.

## Account selection

Selection prefers the account whose remaining weekly capacity is most at risk of being wasted:

1. before the subscription period ends;
2. before the current weekly window resets;
3. before the 5-hour window resets.

Add `--evenly` to rotate among comparable top candidates using recent selection history. It only rebalances accounts with similar pressure, so a low-risk account will not jump ahead of an account that needs usage now:

```bash
oa sync --all --evenly
```

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `OA_AUTH_ISSUER` | `https://auth.openai.com` | Auth issuer endpoint |
| `OA_AUTH_CLIENT_ID` | Embedded in source | OAuth client ID |
| `OA_AUTH_LISTEN` | `127.0.0.1:1455` | Local callback listener |
| `OA_USAGE_BASE_URL` | `https://chatgpt.com/backend-api` | Usage API base URL |

## Noctalia plugin

A local Noctalia plugin lives at `plugins/noctalia/oa-accounts`.

Install it locally:

```bash
go install ./cmd/oa
mkdir -p ~/.config/noctalia/plugins
ln -sfn $(pwd)/plugins/noctalia/oa-accounts ~/.config/noctalia/plugins/oa-accounts
```

Then restart or reload Noctalia, enable **OpenAI Accounts** in the plugin manager if needed, and add its bar widget.

## Development

```bash
go build -o oa ./cmd/oa
go install ./cmd/oa
go test ./...
```
