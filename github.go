package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"time"
)

type GithubPR struct {
	Title       string
	Description string
	URL         string
	Repo        string
	Merged      bool
}

type GithubIssue struct {
	Title       string
	Description string
	URL         string
	Project     string
	Closed      bool
}

type GithubCommit struct {
	Message string
	URL     string
	Repo    string
}

type GithubHistory struct {
	PRs     []GithubPR
	Issues  []GithubIssue
	Commits []GithubCommit
}

type GithubSearchResponse struct {
	Items []struct {
		Title            string    `json:"title"`
		HTMLURL          string    `json:"html_url"`
		Body             string    `json:"body"`
		State            string    `json:"state"`
		PullRequestLinks *struct{} `json:"pull_request"`
		RepositoryURL    string    `json:"repository_url"`
	} `json:"items"`
}

type GithubCommitSearchResponse struct {
	Items []struct {
		HTMLURL    string `json:"html_url"`
		Repository struct {
			FullName string `json:"full_name"`
		} `json:"repository"`
		Commit struct {
			Message string `json:"message"`
		} `json:"commit"`
	} `json:"items"`
}

func getGithubHistory(ctx context.Context, config *Config, username string, days int) (*GithubHistory, error) {
	to := time.Now().Format("2006-01-02")
	from := time.Now().AddDate(0, 0, -days).Format("2006-01-02")

	slog.Info("getting github history", "username", username, "days", days, "from", from, "to", to)

	prs, err := fetchPRs(ctx, config.GitHubToken, username, from, to)
	if err != nil {
		return nil, fmt.Errorf("fetching PRs: %w", err)
	}

	issues, err := fetchIssues(ctx, config.GitHubToken, username, from, to)
	if err != nil {
		return nil, fmt.Errorf("fetching issues: %w", err)
	}

	commits, err := fetchCommits(ctx, config.GitHubToken, username, from, to)
	if err != nil {
		return nil, fmt.Errorf("fetching commits: %w", err)
	}

	slog.Info("github history fetched", "prs", len(prs), "issues", len(issues), "commits", len(commits))

	return &GithubHistory{
		PRs:     prs,
		Issues:  issues,
		Commits: commits,
	}, nil
}

func fetchPRs(ctx context.Context, token, username, from, to string) ([]GithubPR, error) {
	query := fmt.Sprintf("author:%s created:%s..%s type:pr", username, from, to)
	body, err := githubSearch(ctx, token, "issues", query)
	if err != nil {
		return nil, err
	}

	var result GithubSearchResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parsing PR response: %w", err)
	}

	var prs []GithubPR
	for _, item := range result.Items {
		prs = append(prs, GithubPR{
			Title:       item.Title,
			Description: item.Body,
			URL:         item.HTMLURL,
			Repo:        repoFromURL(item.RepositoryURL),
			Merged:      item.State == "closed" && item.PullRequestLinks != nil,
		})
	}
	return prs, nil
}

func fetchIssues(ctx context.Context, token, username, from, to string) ([]GithubIssue, error) {
	query := fmt.Sprintf("assignee:%s closed:%s..%s type:issue", username, from, to)
	body, err := githubSearch(ctx, token, "issues", query)
	if err != nil {
		return nil, err
	}

	var result GithubSearchResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parsing issue response: %w", err)
	}

	var issues []GithubIssue
	for _, item := range result.Items {
		issues = append(issues, GithubIssue{
			Title:       item.Title,
			Description: item.Body,
			URL:         item.HTMLURL,
			Project:     repoFromURL(item.RepositoryURL),
			Closed:      true,
		})
	}
	return issues, nil
}

func fetchCommits(ctx context.Context, token, username, from, to string) ([]GithubCommit, error) {
	query := fmt.Sprintf("author:%s author-date:%s..%s", username, from, to)
	body, err := githubSearch(ctx, token, "commits", query)
	if err != nil {
		return nil, err
	}

	var result GithubCommitSearchResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parsing commit response: %w", err)
	}

	var commits []GithubCommit
	for _, item := range result.Items {
		commits = append(commits, GithubCommit{
			Message: item.Commit.Message,
			URL:     item.HTMLURL,
			Repo:    item.Repository.FullName,
		})
	}
	return commits, nil
}

func githubSearch(ctx context.Context, token, endpoint, query string) ([]byte, error) {
	searchURL := fmt.Sprintf("https://api.github.com/search/%s?q=%s&per_page=100", endpoint, url.QueryEscape(query))

	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	body, err := readBody(resp)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github API returned %d: %s", resp.StatusCode, string(body))
	}

	return body, nil
}

func readBody(resp *http.Response) ([]byte, error) {
	var buf []byte
	buf, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}
	return buf, nil
}

func repoFromURL(repositoryURL string) string {
	// repository_url looks like "https://api.github.com/repos/owner/name"
	if len(repositoryURL) == 0 {
		return ""
	}
	const prefix = "https://api.github.com/repos/"
	if len(repositoryURL) > len(prefix) {
		return repositoryURL[len(prefix):]
	}
	return repositoryURL
}
