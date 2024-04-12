package slackcnr

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/slack-go/slack"
)

type SlackClient interface {
	GetConversationsForUserContext(ctx context.Context, params *slack.GetConversationsForUserParameters) (channels []slack.Channel, nextCursor string, err error)
	GetConversationsContext(ctx context.Context, params *slack.GetConversationsParameters) (channels []slack.Channel, nextCursor string, err error)
}

var _ SlackClient = (*slack.Client)(nil)

type Resolver struct {
	client SlackClient
	opts   resolverOptions
	mu     sync.Mutex
}

type ResolverOption func(*resolverOptions)

type resolverOptions struct {
	searchpublicChannels bool
	cacheStorage         Storage
	batchSize            int
	excludeArchived      bool
	refreshOnCacheMiss   bool
}

// WithSearchPublicChannels enables searching public channels. with conversations.list API.
func WithSearchPublicChannels() ResolverOption {
	return func(o *resolverOptions) {
		o.searchpublicChannels = true
	}
}

// WithCacheStorage sets the cache storage for the resolver. default is in-memory storage.
func WithCacheStorage(storage Storage) ResolverOption {
	return func(o *resolverOptions) {
		o.cacheStorage = storage
	}
}

// WithBatchSize sets the batch size for users.conversations API and conversations.list API limit parameter. default is 1000.
func WithBatchSize(size int) ResolverOption {
	return func(o *resolverOptions) {
		o.batchSize = size
	}
}

// WithExcludeArchived excludes archived channels from the search result.
func WithExcludeArchived() ResolverOption {
	return func(o *resolverOptions) {
		o.excludeArchived = true
	}
}

// WithRefreshOnCacheMiss refreshes the cache storage when a channel is not found in the cache.
func WithRefreshOnCacheMiss() ResolverOption {
	return func(o *resolverOptions) {
		o.refreshOnCacheMiss = true
	}
}

func defaultOptions() resolverOptions {
	return resolverOptions{
		batchSize:    1000,
		cacheStorage: NewInMemoryStorage(24 * time.Hour),
	}
}

// New creates a new resolver with the provided slack client and options.
func New(client SlackClient, optFns ...ResolverOption) *Resolver {
	opts := defaultOptions()
	for _, optFn := range optFns {
		optFn(&opts)
	}
	return &Resolver{
		client: client,
		opts:   opts,
	}
}

// Lookup finds a channel by name.
func (r *Resolver) Lookup(ctx context.Context, channelName string) (*slack.Channel, error) {
	if err := r.prepare(ctx); err != nil {
		return nil, err
	}
	channel, err := r.opts.cacheStorage.GetByChannelName(ctx, channelName)
	if err != nil {
		if !r.opts.refreshOnCacheMiss {
			return nil, err
		}
		if !errors.Is(err, ErrNotFound) {
			return nil, err
		}
		if err := r.Refresh(ctx); err != nil {
			return nil, err
		}
		channel, err = r.opts.cacheStorage.GetByChannelName(ctx, channelName)
	}
	return channel, err
}

func (r *Resolver) prepare(ctx context.Context) error {
	if !r.opts.cacheStorage.NeedRefresh(ctx) {
		return nil
	}
	return r.Refresh(ctx)
}

// Refresh refreshes the cache storage with the latest channels.
func (r *Resolver) Refresh(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	var cursor string
	var sleepTime time.Duration
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(sleepTime):
		default:
		}
		channels, nextCursor, err := r.client.GetConversationsForUserContext(ctx, &slack.GetConversationsForUserParameters{
			Cursor:          cursor,
			Limit:           r.opts.batchSize,
			ExcludeArchived: r.opts.excludeArchived,
		})
		if err != nil {
			var rle *slack.RateLimitedError
			if !errors.As(err, &rle) {
				return err
			}
			if !rle.Retryable() {
				return err
			}
			sleepTime = rle.RetryAfter
			continue
		}
		if err := r.opts.cacheStorage.SetChannels(ctx, channels); err != nil {
			return err
		}
		if nextCursor == "" {
			break
		}
		cursor = nextCursor
	}
	if !r.opts.searchpublicChannels {
		return nil
	}
	cursor = ""
	sleepTime = 0
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(sleepTime):
		default:
		}
		channels, nextCursor, err := r.client.GetConversationsContext(ctx, &slack.GetConversationsParameters{
			Cursor:          cursor,
			Limit:           r.opts.batchSize,
			ExcludeArchived: r.opts.excludeArchived,
		})
		if err != nil {
			var rle *slack.RateLimitedError
			if !errors.As(err, &rle) {
				return err
			}
			if !rle.Retryable() {
				return err
			}
			sleepTime = rle.RetryAfter
			continue
		}
		if err := r.opts.cacheStorage.SetChannels(ctx, channels); err != nil {
			return err
		}
		if nextCursor == "" {
			break
		}
		cursor = nextCursor
	}
	return nil
}
