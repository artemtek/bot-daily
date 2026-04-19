package main

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"github.com/slack-go/slack"
	"github.com/slack-go/slack/socketmode"
)

type Bot struct {
	api    *slack.Client
	socket *socketmode.Client
	cfg    *Config
	store  *Store
}

func NewBot(cfg *Config, store *Store) *Bot {
	api := slack.New(
		cfg.SlackBotToken,
		slack.OptionAppLevelToken(cfg.SlackAppToken),
	)

	return &Bot{
		api:    api,
		socket: socketmode.New(api),
		cfg:    cfg,
		store:  store,
	}
}

func (b *Bot) Run(ctx context.Context) error {
	go b.handleEvents(ctx)
	return b.socket.RunContext(ctx)
}

func (b *Bot) handleEvents(ctx context.Context) {
	for evt := range b.socket.Events {
		switch evt.Type {
		case socketmode.EventTypeSlashCommand:
			cmd, _ := evt.Data.(slack.SlashCommand)
			switch cmd.Command {
			case "/daily":
				b.handleDaily(ctx, evt, cmd)
			case "/monthly":
				b.handleMonthly(ctx, evt, cmd)
			case "/subscribe":
				b.handleSubscribe(ctx, evt, cmd)
			}
		}
	}
}

func (b *Bot) handleDaily(ctx context.Context, evt socketmode.Event, cmd slack.SlashCommand) {
	args := strings.TrimSpace(cmd.Text)
	parts := strings.Fields(args)

	slog.Info("daily command", "user", cmd.UserID, "args", args)

	if args == "help" {
		b.handleHelp(evt)
		return
	}

	user, known := b.store.Get(cmd.UserID)

	// single word that's not a number — treat as github username
	if len(parts) == 1 {
		if _, err := strconv.Atoi(parts[0]); err != nil {
			if err := b.store.SetGitHub(cmd.UserID, parts[0]); err != nil {
				b.ack(evt, "Failed to save. Try again.")
				return
			}
			slog.Info("github username set", "user", cmd.UserID, "github", parts[0])
			b.ack(evt, fmt.Sprintf("GitHub username set to *%s*. Generating today's report...", parts[0]))
			go b.runPipelineFor(ctx, cmd.UserID, 1, false)
			return
		}
	}

	if !known || user.GitHubUsername == "" {
		b.ack(evt, "I don't know your GitHub username yet. Use `/daily <github-username>` to set it.")
		return
	}

	// parse days
	days := 1
	if len(parts) == 1 {
		if n, err := strconv.Atoi(parts[0]); err == nil && n > 0 && n <= 30 {
			days = n
		}
	}

	b.ack(evt, fmt.Sprintf("Generating summary for the last %d day(s)...", days))
	go b.runPipelineFor(ctx, cmd.UserID, days, false)
}

func (b *Bot) handleMonthly(ctx context.Context, evt socketmode.Event, cmd slack.SlashCommand) {
	slog.Info("monthly command", "user", cmd.UserID)

	user, known := b.store.Get(cmd.UserID)
	if !known || user.GitHubUsername == "" {
		b.ack(evt, "I don't know your GitHub username yet. Use `/daily <github-username>` to set it.")
		return
	}

	b.ack(evt, "Generating PSR (monthly status report)...")
	go b.runPipelineFor(ctx, cmd.UserID, 30, true)
}

func (b *Bot) handleSubscribe(ctx context.Context, evt socketmode.Event, cmd slack.SlashCommand) {
	args := strings.TrimSpace(cmd.Text)

	slog.Info("subscribe command", "user", cmd.UserID, "args", args)

	user, known := b.store.Get(cmd.UserID)
	if (!known || user.GitHubUsername == "") && args != "status" && !strings.HasPrefix(args, "github ") {
		b.ack(evt, "I don't know your GitHub username yet. Use `/daily <github-username>` first.")
		return
	}

	parts := strings.Fields(args)

	switch {
	case len(parts) == 2 && parts[0] == "github":
		if err := b.store.SetGitHub(cmd.UserID, parts[1]); err != nil {
			b.ack(evt, "Failed to save. Try again.")
			return
		}
		b.ack(evt, fmt.Sprintf("GitHub username updated to *%s*.", parts[1]))

	case args == "daily":
		if err := b.store.Subscribe(cmd.UserID); err != nil {
			b.ack(evt, "Failed to subscribe. Try again.")
			return
		}
		b.ack(evt, "Subscribed to daily updates!")

	case args == "monthly":
		if err := b.store.PSRSubscribe(cmd.UserID); err != nil {
			b.ack(evt, "Failed to subscribe. Try again.")
			return
		}
		b.ack(evt, "Subscribed to monthly PSR reports!")

	case args == "stop daily":
		if err := b.store.Unsubscribe(cmd.UserID); err != nil {
			b.ack(evt, "Failed to unsubscribe. Try again.")
			return
		}
		b.ack(evt, "Unsubscribed from daily updates.")

	case args == "stop monthly":
		if err := b.store.PSRUnsubscribe(cmd.UserID); err != nil {
			b.ack(evt, "Failed to unsubscribe. Try again.")
			return
		}
		b.ack(evt, "Unsubscribed from monthly PSR reports.")

	case args == "status":
		b.handleStatus(evt, cmd.UserID)

	default:
		b.ack(evt, strings.Join([]string{
			"`/subscribe daily` — get daily scheduled updates",
			"`/subscribe monthly` — get monthly PSR reports",
			"`/subscribe stop daily` — stop daily updates",
			"`/subscribe stop monthly` — stop monthly PSR",
			"`/subscribe github <username>` — change your GitHub username",
			"`/subscribe status` — check your settings",
		}, "\n"))
	}
}

func (b *Bot) handleStatus(evt socketmode.Event, slackUserID string) {
	user, ok := b.store.Get(slackUserID)
	if !ok || user.GitHubUsername == "" {
		b.ack(evt, "You haven't set up yet. Use `/daily <github-username>` to get started.")
		return
	}

	daily := "not subscribed"
	if user.Subscribed && user.SubscribedAt != nil {
		daily = fmt.Sprintf("subscribed since %s", user.SubscribedAt.Format("Jan 2, 2006"))
	}

	psr := "not subscribed"
	if user.PSRSubscribed && user.PSRSubscribedAt != nil {
		psr = fmt.Sprintf("subscribed since %s", user.PSRSubscribedAt.Format("Jan 2, 2006"))
	}

	b.ack(evt, fmt.Sprintf("GitHub: *%s*\nDaily: %s\nPSR: %s", user.GitHubUsername, daily, psr))
}

func (b *Bot) handleHelp(evt socketmode.Event) {
	b.ack(evt, strings.Join([]string{
		"`/daily` — summary for today",
		"`/daily <N>` — summary for the last N days (max 30)",
		"`/daily <github-username>` — set your GitHub username",
		"`/monthly` — generate monthly Project Status Report",
		"`/subscribe daily` — get daily updates automatically",
		"`/subscribe monthly` — get monthly PSR automatically",
		"`/subscribe stop daily` — stop daily updates",
		"`/subscribe stop monthly` — stop monthly PSR",
		"`/subscribe status` — check your settings",
		"`/daily help` — show this message",
	}, "\n"))
}

func (b *Bot) runPipelineFor(ctx context.Context, slackUserID string, days int, monthly bool) {
	user, _ := b.store.Get(slackUserID)
	b.runPipeline(ctx, slackUserID, user.GitHubUsername, days, monthly)
}

func (b *Bot) runPipeline(ctx context.Context, slackUserID, ghUsername string, days int, monthly bool) {
	history, err := getGithubHistory(ctx, b.cfg, ghUsername, days)
	if err != nil {
		slog.Error("github fetch failed", "user", slackUserID, "error", err)
		b.sendDM(slackUserID, "Failed to fetch GitHub data. Try again later.")
		return
	}

	var summary string
	if monthly {
		summary, err = summarizePSR(ctx, b.cfg, history)
	} else {
		summary, err = summarizeDaily(ctx, b.cfg, history)
	}

	if err != nil {
		slog.Error("LLM summary failed, using fallback", "user", slackUserID, "error", err)
		summary = fallbackSummary(history)
	}

	b.sendDM(slackUserID, summary)
}

func (b *Bot) sendDM(slackUserID, text string) {
	channel, _, _, err := b.api.OpenConversation(&slack.OpenConversationParameters{
		Users: []string{slackUserID},
	})
	if err != nil {
		slog.Error("failed to open DM", "user", slackUserID, "error", err)
		return
	}

	_, _, err = b.api.PostMessage(channel.ID,
		slack.MsgOptionText(text, false),
		slack.MsgOptionDisableLinkUnfurl(),
	)
	if err != nil {
		slog.Error("failed to send DM", "user", slackUserID, "error", err)
	}
}

func (b *Bot) ack(evt socketmode.Event, text string) {
	b.socket.Ack(*evt.Request, map[string]interface{}{
		"response_type": "ephemeral",
		"text":          text,
	})
}
