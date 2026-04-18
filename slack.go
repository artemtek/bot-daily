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
			b.handleSlashCommand(ctx, evt, cmd)
		}
	}
}

func (b *Bot) handleSlashCommand(ctx context.Context, evt socketmode.Event, cmd slack.SlashCommand) {
	args := strings.TrimSpace(cmd.Text)
	parts := strings.Fields(args)

	slog.Info("command received", "user", cmd.UserID, "args", args)

	switch {
	case len(parts) >= 2 && parts[0] == "subscribe":
		b.handleSubscribe(evt, cmd.UserID, parts[1])

	case args == "unsubscribe":
		b.handleUnsubscribe(evt, cmd.UserID)

	case args == "status":
		b.handleStatus(evt, cmd.UserID)

	case args == "monthly":
		b.ack(evt, "Generating monthly report...")
		go b.runReport(ctx, cmd.UserID, 30, true)

	default:
		days := 1
		if len(parts) == 1 {
			if n, err := strconv.Atoi(parts[0]); err == nil && n > 0 && n <= 30 {
				days = n
			}
		}
		b.ack(evt, fmt.Sprintf("Generating summary for the last %d day(s)...", days))
		go b.runReport(ctx, cmd.UserID, days, false)
	}
}

func (b *Bot) handleSubscribe(evt socketmode.Event, slackUserID, ghUsername string) {
	if err := b.store.Add(slackUserID, ghUsername); err != nil {
		slog.Error("subscribe failed", "user", slackUserID, "error", err)
		b.ack(evt, "Failed to subscribe. Try again.")
		return
	}

	slog.Info("user subscribed", "user", slackUserID, "github", ghUsername)
	b.ack(evt, fmt.Sprintf("Subscribed! GitHub user: %s. You'll receive daily updates.", ghUsername))
}

func (b *Bot) handleUnsubscribe(evt socketmode.Event, slackUserID string) {
	if err := b.store.Remove(slackUserID); err != nil {
		slog.Error("unsubscribe failed", "user", slackUserID, "error", err)
		b.ack(evt, "Failed to unsubscribe. Try again.")
		return
	}

	slog.Info("user unsubscribed", "user", slackUserID)
	b.ack(evt, "Unsubscribed. You won't receive daily updates anymore.")
}

func (b *Bot) handleStatus(evt socketmode.Event, slackUserID string) {
	sub, ok := b.store.Get(slackUserID)
	if !ok {
		b.ack(evt, "You're not subscribed. Use `/daily subscribe <github-username>` to start.")
		return
	}

	b.ack(evt, fmt.Sprintf("Subscribed since %s. GitHub user: %s",
		sub.SubscribedAt.Format("Jan 2, 2006"), sub.GitHubUsername))
}

func (b *Bot) runReport(ctx context.Context, slackUserID string, days int, monthly bool) {
	sub, ok := b.store.Get(slackUserID)
	if !ok {
		b.sendDM(slackUserID, "You're not subscribed. Use `/daily subscribe <github-username>` first.")
		return
	}

	b.runPipeline(ctx, slackUserID, sub.GitHubUsername, days, monthly)
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
		summary, err = summarizeMonthly(ctx, b.cfg, history)
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
