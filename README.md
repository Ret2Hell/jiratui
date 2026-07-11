# jiratui

[![GitHub Release](https://img.shields.io/github/v/release/Ret2Hell/jiratui?style=flat&color=blue)](https://github.com/Ret2Hell/jiratui/releases/latest)
[![CI](https://img.shields.io/github/actions/workflow/status/Ret2Hell/jiratui/ci.yml?label=CI)](https://github.com/Ret2Hell/jiratui/actions/workflows/ci.yml)
[![Go](https://img.shields.io/badge/Go-1.26-blue)](go.mod)
[![Platform](https://img.shields.io/badge/macOS_%7C_Linux_%7C_Windows-supported-lightgrey)](https://github.com/Ret2Hell/jiratui/releases/latest)
[![License](https://img.shields.io/badge/license-see_LICENSE-green)](LICENSE)

A minimal Jira Cloud TUI for active sprint tickets, task creation and editing, status changes, story points, and IONOS daily report drafts.

## Installation

### Go

Run without installing:

```bash
go run github.com/Ret2Hell/jiratui/cmd/jiratui@latest
```

Install globally:

```bash
go install github.com/Ret2Hell/jiratui/cmd/jiratui@latest
```

### Arch Linux AUR

Install `jiratui` from the AUR:

```bash
yay -S jiratui-bin
```

## Setup

Start the app to open setup when no config exists:

```bash
jiratui
```

Setup has two steps:

1. **Jira**
   - **Base URL**: your Jira site, e.g. `https://company.atlassian.net`
   - **Email**: your Atlassian/Jira login email
   - **API token**: create one at <https://id.atlassian.com/manage-profile/security/api-tokens>
   - **Project key**: the prefix in issue IDs, e.g. `OPS` from `OPS-123`

2. **IONOS mail drafts**
   - **Mailbox email**: full IONOS mailbox address
   - **Mailbox password**: mailbox password, not the IONOS control-panel password
   - **Recipients**: comma-separated report recipients

Setup keys: `tab` next, `shift+tab` previous, `enter` continue/save, `q` quit.

Secrets are stored in the OS keyring. You can also use:

```bash
JIRATUI_JIRA_TOKEN=...
JIRATUI_MAIL_PASSWORD=...
```

Config defaults to `~/.config/jiratui/config.yaml`. Reopen setup with:

```bash
jiratui --setup
```

## Keybindings

| Key | Action |
| --- | --- |
| `q` | quit |
| `?` | help |
| `r` | refresh tickets |
| `/` | filter tickets |
| `n` | create task |
| `e` / `R` | edit selected task |
| `enter` | set story points |
| `t` | move to To Do |
| `p` / `i` | move to In Progress |
| `x` / `d` | move to Done |
| `m` | open daily report draft |
| `ctrl+s` | save report draft |
| `esc` | cancel/close |
