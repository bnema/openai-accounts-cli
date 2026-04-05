# openai-accounts-cli (oa)

**oa** tracks usage and manages auth across multiple OpenAI accounts.

## What it does

- Stores account metadata and secret references in `~/.codex/accounts.toml`
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
|- opencode                        Manage OpenCode integration
|  |- doctor                       Check OpenCode integration
|  |- handle --json                Handle OpenCode requests via JSON stdin/stdout
|  |- install                      Install OpenCode integration
|  |- install-systemd              Install a systemd user timer for OpenCode sync
|  `- sync                         Sync OpenCode auth
|- usage [--account <id>] [--json] Fetch and display account usage limits (alias: `status`)
`- version                         Print version information
```

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
