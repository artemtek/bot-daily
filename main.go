package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/robfig/cron/v3"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	cfg, err := loadConfig()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	store, err := NewStore(cfg.DataDir)
	if err != nil {
		slog.Error("failed to init store", "error", err)
		os.Exit(1)
	}

	bot := NewBot(cfg, store)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	c := cron.New()

	_, err = c.AddFunc(cfg.Schedule, func() {
		slog.Info("daily cron triggered")
		runScheduled(ctx, bot, 1, false)
	})
	if err != nil {
		slog.Error("invalid daily schedule", "schedule", cfg.Schedule, "error", err)
		os.Exit(1)
	}

	_, err = c.AddFunc(cfg.MonthlySchedule, func() {
		slog.Info("monthly cron triggered")
		runScheduled(ctx, bot, 30, true)
	})
	if err != nil {
		slog.Error("invalid monthly schedule", "schedule", cfg.MonthlySchedule, "error", err)
		os.Exit(1)
	}

	c.Start()
	slog.Info("cron started", "daily", cfg.Schedule, "monthly", cfg.MonthlySchedule)

	if cfg.RunOnStart {
		slog.Info("running daily pipeline on startup")
		go runScheduled(ctx, bot, 1, false)
	}

	go func() {
		sigs := make(chan os.Signal, 1)
		signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
		sig := <-sigs
		slog.Info("shutting down", "signal", sig.String())
		c.Stop()
		cancel()
	}()

	slog.Info("bot starting")
	if err := bot.Run(ctx); err != nil {
		slog.Error("bot stopped", "error", err)
		os.Exit(1)
	}
}

func runScheduled(ctx context.Context, bot *Bot, days int, monthly bool) {
	subs := bot.store.ListAll()
	slog.Info("running scheduled pipeline", "subscribers", len(subs), "days", days, "monthly", monthly)

	for slackUserID, sub := range subs {
		slog.Info("processing subscriber", "user", slackUserID, "github", sub.GitHubUsername)
		bot.runPipeline(ctx, slackUserID, sub.GitHubUsername, days, monthly)
	}
}
