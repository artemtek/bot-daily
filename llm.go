package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
)

func formatHistory(h *GithubHistory) string {
	var b strings.Builder

	b.WriteString("## Pull Requests\n")
	if len(h.PRs) == 0 {
		b.WriteString("None\n")
	}
	for _, pr := range h.PRs {
		status := "open"
		if pr.Merged {
			status = "merged"
		}
		b.WriteString(fmt.Sprintf("- [%s] %s (%s) %s\n", status, pr.Title, pr.Repo, pr.URL))
	}

	b.WriteString("\n## Commits\n")
	if len(h.Commits) == 0 {
		b.WriteString("None\n")
	}
	for _, c := range h.Commits {
		msg := strings.Split(c.Message, "\n")[0] // first line only
		b.WriteString(fmt.Sprintf("- %s (%s)\n", msg, c.Repo))
	}

	b.WriteString("\n## Issues Resolved\n")
	if len(h.Issues) == 0 {
		b.WriteString("None\n")
	}
	for _, issue := range h.Issues {
		b.WriteString(fmt.Sprintf("- %s (%s) %s\n", issue.Title, issue.Project, issue.URL))
	}

	return b.String()
}

func summarizeDaily(ctx context.Context, cfg *Config, history *GithubHistory) (string, error) {
	systemPrompt := `You are writing a daily accomplishments update for a developer. You will receive their raw GitHub activity (PRs, commits, issues). Your job is to distill it into a clean, human-readable update.

Rules:
- Group accomplishments by project/repo
- Each project section starts with "Project: <name>" (use the repo name, short form)
- Under each project, list "Accomplishments:" as short bullet points
- Describe WHAT was done in plain language, not git-speak. Turn PR titles and commit messages into readable accomplishments.
- Merge related PRs and commits into a single bullet point when they describe the same work
- Do NOT include URLs, SHAs, branch names, or technical metadata
- Do NOT include section headers like "PRs" or "Commits" — just accomplishments
- Keep each bullet to one line, concise but descriptive enough to understand the work
- Skip empty projects`

	return callLLM(ctx, cfg, systemPrompt, formatHistory(history))
}

func summarizeMonthly(ctx context.Context, cfg *Config, history *GithubHistory) (string, error) {
	systemPrompt := `You are writing a monthly accomplishments report for a developer. You will receive their raw GitHub activity (PRs, commits, issues) for the past month. Your job is to distill it into a clean, high-level report.

Rules:
- Group accomplishments by project/repo
- Each project section starts with "Project: <name>" (use the repo name, short form)
- Under each project, list "Accomplishments:" as bullet points summarizing the month's work
- Merge related items — a month of commits on one feature becomes a single bullet
- After all projects, add a "Summary:" section with 2-3 sentences about overall themes and impact
- Describe WHAT was done in plain language, not git-speak
- Do NOT include URLs, SHAs, branch names, or technical metadata
- Keep it concise but comprehensive enough to reflect a full month of work`

	return callLLM(ctx, cfg, systemPrompt, formatHistory(history))
}

type llmRequest struct {
	Messages []llmMessage `json:"messages"`
}

type llmMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type llmResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

func callLLM(ctx context.Context, cfg *Config, systemPrompt, userContent string) (string, error) {
	reqURL := fmt.Sprintf("%s/openai/deployments/%s/chat/completions?api-version=%s",
		cfg.AzureOpenAIEndpoint, cfg.AzureOpenAIDeployment, cfg.AzureOpenAIAPIVersion)

	reqBody := llmRequest{
		Messages: []llmMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userContent},
		},
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshaling LLM request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", reqURL, bytes.NewReader(jsonBody))
	if err != nil {
		return "", fmt.Errorf("creating LLM request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("api-key", cfg.AzureOpenAIKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("calling LLM: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading LLM response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("LLM returned %d: %s", resp.StatusCode, string(body))
	}

	var result llmResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("parsing LLM response: %w", err)
	}

	if len(result.Choices) == 0 {
		return "", fmt.Errorf("LLM returned no choices")
	}

	slog.Info("LLM summary generated", "length", len(result.Choices[0].Message.Content))
	return result.Choices[0].Message.Content, nil
}

func fallbackSummary(h *GithubHistory) string {
	return fmt.Sprintf("*Daily Summary (LLM unavailable)*\n\n%s", formatHistory(h))
}
