package main

import (
	"fmt"
	"os"
)

type Config struct {
	GitHubToken           string
	SlackBotToken         string
	SlackAppToken         string
	AzureOpenAIEndpoint   string
	AzureOpenAIKey        string
	AzureOpenAIDeployment string
	AzureOpenAIAPIVersion string
	Schedule              string
	MonthlySchedule       string
	DataDir               string
	RunOnStart            bool
}

func loadConfig() (*Config, error) {
	cfg := &Config{
		Schedule:        getEnvDefault("SCHEDULE", "0 17 * * 1-5"),      // 5:00 PM on weekdays
		MonthlySchedule: getEnvDefault("MONTHLY_SCHEDULE", "0 9 1 * *"), // 9:00 AM on the first day of the month
		DataDir:         getEnvDefault("DATA_DIR", "./data"),
		RunOnStart:      os.Getenv("RUN_ON_START") == "true",
	}

	required := map[string]*string{
		"GITHUB_TOKEN":             &cfg.GitHubToken,
		"SLACK_BOT_TOKEN":          &cfg.SlackBotToken,
		"SLACK_APP_TOKEN":          &cfg.SlackAppToken,
		"AZURE_OPENAI_ENDPOINT":    &cfg.AzureOpenAIEndpoint,
		"AZURE_OPENAI_KEY":         &cfg.AzureOpenAIKey,
		"AZURE_OPENAI_DEPLOYMENT":  &cfg.AzureOpenAIDeployment,
		"AZURE_OPENAI_API_VERSION": &cfg.AzureOpenAIAPIVersion,
	}

	for env, dest := range required {
		val := os.Getenv(env)
		if val == "" {
			return nil, fmt.Errorf("missing required environment variable: %s", env)
		}
		*dest = val
	}

	return cfg, nil
}

func getEnvDefault(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}
