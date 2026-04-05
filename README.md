# openai-accounts-cli (oa)

**oa** tracks usage and manages auth across multiple OpenAI accounts.

## What it does

- Stores per-account auth in `~/.codex/accounts.toml`
- Stores secrets via `pass`, with file fallback at `~/.codex/secrets`
- Supports API key and ChatGPT OAuth auth
- Fetches daily and weekly usage limits from OpenAI
- Recommends which account to use based on weekly pressure
- Shows subscription renewal countdowns
- Syncs ChatGPT OAuth credentials into OpenCode

## Install

```bash
go install github.com/bnema/openai-accounts-cli/cmd/oa@latest
# or from a clone
make install
```

## Commands

| Command | Description |
|---------|-------------|
| `oa account list` | List configured accounts |
| `oa account remove <id>` | Remove an account and its credentials |
| `oa auth check [--account <id>]` | Verify OAuth session validity for ChatGPT accounts |
| `oa auth login browser [--account <id>]` | Start browser OAuth login flow |
| `oa auth login device [--account <id>]` | Start device OAuth login flow *(not implemented)* |
| `oa auth set [--account <id>]` | Set account credentials manually (`--method`, `--secret-key`, `--secret-value`) |
| `oa auth remove --account <id>` | Remove account credentials |
| `oa usage [--account <id>] [--json]` | Fetch and display usage limits (alias: `status`) |
| `oa opencode install` | Install the OpenCode plugin shim |
| `oa opencode doctor` | Check OpenCode integration state |
| `oa opencode sync` | Sync OpenCode auth with the best eligible account |
| `oa opencode install-systemd` | Install a systemd user timer for periodic sync |
| `oa opencode handle --json` | Handle OpenCode recovery requests via JSON stdin/stdout |
| `oa version` | Print version |

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `OA_AUTH_ISSUER` | `https://auth.openai.com` | Auth issuer endpoint |
| `OA_AUTH_CLIENT_ID` | Embedded in source | OAuth client ID |
| `OA_AUTH_LISTEN` | `127.0.0.1:1455` | Local callback listener |
| `OA_USAGE_BASE_URL` | `https://chatgpt.com/backend-api` | Usage API base URL |

## Development

```bash
go test ./...
make build
make install
```
