# bot-daily

Slack bot that generates personalized daily and monthly GitHub activity summaries using Azure OpenAI. Runs in a ~12 MB Docker container with no inbound ports (Slack Socket Mode).

## What it does

- Fetches your PRs, commits, and resolved issues from GitHub
- Sends them to Azure OpenAI to produce a clean accomplishments summary
- Posts the summary to you as a Slack DM — on schedule or on demand
- Any Slack user can subscribe with their GitHub username

## Commands

| Command | Description |
|---|---|
| `/daily subscribe <github-username>` | Subscribe to daily updates |
| `/daily unsubscribe` | Stop receiving updates |
| `/daily status` | Check subscription info |
| `/daily` | On-demand summary for today |
| `/daily <N>` | Summary for the last N days (max 30) |
| `/daily monthly` | Monthly report |

## Setup

### 1. Slack App

1. Create a Slack app at [api.slack.com/apps](https://api.slack.com/apps)
2. Enable **Socket Mode** and generate an App-Level Token (`xapp-...`) with `connections:write` scope
3. Add Bot Token Scopes: `chat:write`, `commands`, `im:write`
4. Create a slash command: `/daily`
5. Under App Home, enable "Messages Tab" and "Allow users to send Slash commands and messages from the messages tab"
6. Install the app to your workspace and copy the Bot User OAuth Token (`xoxb-...`)

### 2. GitHub Token

Create a [Personal Access Token](https://github.com/settings/tokens) with `repo` scope.

### 3. Configure

```bash
cp .env.example .env
# Fill in your real values
```

### 4. Run

```bash
docker compose up --build -d
```

View logs:

```bash
docker compose logs -f
```

## Environment Variables

| Variable | Required | Default | Description |
|---|---|---|---|
| `GITHUB_TOKEN` | Yes | — | GitHub personal access token |
| `SLACK_BOT_TOKEN` | Yes | — | Slack bot token (`xoxb-...`) |
| `SLACK_APP_TOKEN` | Yes | — | Slack app-level token (`xapp-...`) |
| `AZURE_OPENAI_ENDPOINT` | Yes | — | Azure OpenAI endpoint URL |
| `AZURE_OPENAI_KEY` | Yes | — | Azure OpenAI API key |
| `AZURE_OPENAI_DEPLOYMENT` | Yes | — | Azure OpenAI deployment name |
| `AZURE_OPENAI_API_VERSION` | Yes | — | Azure OpenAI API version |
| `SCHEDULE` | No | `0 17 * * 1-5` | Daily cron schedule |
| `MONTHLY_SCHEDULE` | No | `0 9 1 * *` | Monthly cron schedule |
| `DATA_DIR` | No | `./data` | Directory for subscriber data |
| `RUN_ON_START` | No | `false` | Run daily pipeline on startup |

## License

GPL-2.0
