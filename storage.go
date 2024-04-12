package slackcnr

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/slack-go/slack"
)

var ErrNotFound = errors.New("channel not found")

// Storage defines the interface for caching slack channels.
type Storage interface {
	SetChannels(ctx context.Context, channels []slack.Channel) error
	GetByChannelName(ctx context.Context, channelName string) (*slack.Channel, error)
	NeedRefresh(ctx context.Context) bool
}

type InMemoryStorage struct {
	mu             sync.RWMutex
	channels       map[string]slack.Channel
	namesById      map[string]string
	lastSetTime    time.Time
	expredDuration time.Duration
}

// NewInMemoryStorage creates a new in-memory storage. if expredDuration is 0, it never expires.
func NewInMemoryStorage(expredDuration time.Duration) *InMemoryStorage {
	return &InMemoryStorage{
		expredDuration: expredDuration,
		channels:       make(map[string]slack.Channel),
		namesById:      make(map[string]string),
	}
}

func (s *InMemoryStorage) SetChannels(ctx context.Context, channels []slack.Channel) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, channel := range channels {
		s.channels[channel.ID] = channel
		s.namesById[channel.Name] = channel.ID
	}

	s.lastSetTime = time.Now()
	return nil
}

func (s *InMemoryStorage) GetByChannelName(ctx context.Context, channelName string) (*slack.Channel, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	id, ok := s.namesById[channelName]
	if !ok {
		return nil, ErrNotFound
	}

	channel, ok := s.channels[id]
	if !ok {
		return nil, ErrNotFound
	}

	return &channel, nil
}

func (s *InMemoryStorage) NeedRefresh(ctx context.Context) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.lastSetTime.IsZero() {
		return true
	}
	if s.expredDuration == 0 {
		return false
	}
	return time.Since(s.lastSetTime) > s.expredDuration
}
