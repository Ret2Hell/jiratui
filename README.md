# lazyjira

A minimal Jira Cloud TUI for active sprint tickets, task creation, status changes, story points, and IONOS daily report drafts.

## Installation

Run without installing:

```bash
go run github.com/Ret2Hell/lazyjira/cmd/lazyjira@latest
```

Install globally:

```bash
go install github.com/Ret2Hell/lazyjira/cmd/lazyjira@latest
```

## Setup

Start the app to open setup when no config exists:

```bash
lazyjira
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
LAZYJIRA_JIRA_TOKEN=...
LAZYJIRA_MAIL_PASSWORD=...
```

Config defaults to `~/.config/lazyjira/config.yaml`. Reopen setup with:

```bash
lazyjira --setup
```

## Keybindings

| Key | Action |
|---|---|
| `q` | quit |
| `?` | help |
| `r` | refresh tickets |
| `/` | filter tickets |
| `n` | create task |
| `enter` | set story points |
| `t` | move to To Do |
| `p` / `i` | move to In Progress |
| `x` / `d` | move to Done |
| `m` / `R` | open daily report draft |
| `ctrl+s` | save report draft |
| `esc` | cancel/close |
