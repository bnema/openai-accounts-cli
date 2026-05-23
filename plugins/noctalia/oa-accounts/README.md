# OpenAI Accounts for Noctalia

Companion Noctalia plugin for `oa`.

## Local install

Build or install the CLI first:

```bash
go install ./cmd/oa
```

Install the plugin locally with a symlink:

```bash
mkdir -p ~/.config/noctalia/plugins
ln -sfn /absolute/path/to/openai-accounts-cli/plugins/noctalia/oa-accounts ~/.config/noctalia/plugins/oa-accounts
```

Then:

1. restart or reload Noctalia
2. enable **OpenAI Accounts** in the plugin manager if it is disabled
3. add the plugin's bar widget to your bar
4. click the widget to open the panel

## CLI contract used

- `oa noctalia snapshot --json`
- `oa noctalia snapshot --json --refresh`
- `oa sync opencode --json`
- `oa sync codex --json`
- `oa sync pi --json`
- `oa sync --all --json`

The QML layer only renders the snapshot and triggers returned sync commands.
