package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type User struct {
	GitHubUsername  string     `json:"github_username"`
	Subscribed     bool       `json:"subscribed"`
	SubscribedAt   *time.Time `json:"subscribed_at,omitempty"`
	PSRSubscribed  bool       `json:"psr_subscribed"`
	PSRSubscribedAt *time.Time `json:"psr_subscribed_at,omitempty"`
}

type Store struct {
	mu    sync.RWMutex
	path  string
	users map[string]User // key: Slack user ID
}

func NewStore(dataDir string) (*Store, error) {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("creating data dir: %w", err)
	}

	s := &Store{
		path:  filepath.Join(dataDir, "subscribers.json"),
		users: make(map[string]User),
	}

	if err := s.load(); err != nil {
		return nil, err
	}

	return s, nil
}

func (s *Store) load() error {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("reading subscribers: %w", err)
	}

	if err := json.Unmarshal(data, &s.users); err != nil {
		return fmt.Errorf("parsing subscribers: %w", err)
	}

	return nil
}

func (s *Store) save() error {
	data, err := json.MarshalIndent(s.users, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling subscribers: %w", err)
	}

	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return fmt.Errorf("writing subscribers: %w", err)
	}

	if err := os.Rename(tmp, s.path); err != nil {
		return fmt.Errorf("renaming subscribers file: %w", err)
	}

	return nil
}

func (s *Store) SetGitHub(slackUserID, githubUsername string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	u := s.users[slackUserID]
	u.GitHubUsername = githubUsername
	s.users[slackUserID] = u

	return s.save()
}

func (s *Store) Subscribe(slackUserID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	u := s.users[slackUserID]
	u.Subscribed = true
	now := time.Now()
	u.SubscribedAt = &now
	s.users[slackUserID] = u

	return s.save()
}

func (s *Store) Unsubscribe(slackUserID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	u := s.users[slackUserID]
	u.Subscribed = false
	u.SubscribedAt = nil
	s.users[slackUserID] = u

	return s.save()
}

func (s *Store) PSRSubscribe(slackUserID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	u := s.users[slackUserID]
	u.PSRSubscribed = true
	now := time.Now()
	u.PSRSubscribedAt = &now
	s.users[slackUserID] = u

	return s.save()
}

func (s *Store) PSRUnsubscribe(slackUserID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	u := s.users[slackUserID]
	u.PSRSubscribed = false
	u.PSRSubscribedAt = nil
	s.users[slackUserID] = u

	return s.save()
}

func (s *Store) Get(slackUserID string) (User, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	u, ok := s.users[slackUserID]
	return u, ok
}

func (s *Store) ListSubscribed() map[string]User {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[string]User)
	for k, v := range s.users {
		if v.Subscribed {
			result[k] = v
		}
	}
	return result
}

func (s *Store) ListPSRSubscribed() map[string]User {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[string]User)
	for k, v := range s.users {
		if v.PSRSubscribed {
			result[k] = v
		}
	}
	return result
}
