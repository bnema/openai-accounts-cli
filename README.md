# openai-accounts-cli (oa)

**oa** tracks usage and rotates auth across multiple OpenAI accounts.

## What it does

- Stores per-account auth in `~/.codex/accounts.toml`
- Stores secrets via `pass`, with file fallback at `~/.codex/secrets`
- Supports API key and ChatGPT OAuth auth
- Fetches daily and weekly usage limits from OpenAI
- Recommends which account to use based on weekly pressure
- Shows subscription renewal countdowns
- Rotates credentials into external tools (opencode)

## Install

```bash
go install github.com/bnema/openai-accounts-cli/cmd/oa@latest
```

## Commands

| Command | Description | Flags |
|---------|-------------|-------|
| `oa account list` | List configured accounts | |
| `oa account remove <id>` | Remove an account and its credentials | |
| `oa auth check` | Verify OAuth session validity for ChatGPT accounts | `--account <id>` |
| `oa auth login browser` | Start browser OAuth login flow | `--account <id>` |
| `oa auth login device` | Start device OAuth login flow *(not implemented)* | `--account <id>` |
| `oa auth set` | Set account credentials manually | `--account <id>` `--method api_key\|chatgpt` `--secret-key <key>` `--secret-value <val>` |
| `oa auth remove` | Remove account credentials | `--account <id>` |
| `oa usage` | Fetch and display usage limits (alias: `status`) | `--account <id>` `--json` |
| `oa rotate opencode` | Patch opencode's codex auth with the best available account | |
| `oa version` | Print version | |

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
go build -o oa ./cmd/oa
```
