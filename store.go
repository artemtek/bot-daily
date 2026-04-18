package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Subscriber struct {
	GitHubUsername string    `json:"github_username"`
	SubscribedAt  time.Time `json:"subscribed_at"`
}

type Store struct {
	mu   sync.RWMutex
	path string
	subs map[string]Subscriber // key: Slack user ID
}

func NewStore(dataDir string) (*Store, error) {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("creating data dir: %w", err)
	}

	s := &Store{
		path: filepath.Join(dataDir, "subscribers.json"),
		subs: make(map[string]Subscriber),
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

	if err := json.Unmarshal(data, &s.subs); err != nil {
		return fmt.Errorf("parsing subscribers: %w", err)
	}

	return nil
}

func (s *Store) save() error {
	data, err := json.MarshalIndent(s.subs, "", "  ")
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

func (s *Store) Add(slackUserID, githubUsername string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.subs[slackUserID] = Subscriber{
		GitHubUsername: githubUsername,
		SubscribedAt:  time.Now(),
	}

	return s.save()
}

func (s *Store) Remove(slackUserID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.subs, slackUserID)
	return s.save()
}

func (s *Store) Get(slackUserID string) (Subscriber, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sub, ok := s.subs[slackUserID]
	return sub, ok
}

func (s *Store) ListAll() map[string]Subscriber {
	s.mu.RLock()
	defer s.mu.RUnlock()

	copy := make(map[string]Subscriber, len(s.subs))
	for k, v := range s.subs {
		copy[k] = v
	}
	return copy
}
