package slackcnr_test

import (
	"context"
	"testing"
	"time"

	"github.com/mashiike/slackcnr"
	"github.com/slack-go/slack"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type mockSlackClient struct {
	t *testing.T
	mock.Mock
}

func (m *mockSlackClient) GetConversationsForUserContext(ctx context.Context, params *slack.GetConversationsForUserParameters) (channels []slack.Channel, nextCursor string, err error) {
	args := m.Called(ctx, params)
	var ok bool
	channels, ok = args.Get(0).([]slack.Channel)
	if !ok {
		m.t.Error("failed to cast channels")
	}
	nextCursor = args.String(1)
	err = args.Error(2)
	return channels, nextCursor, err
}

func (m *mockSlackClient) GetConversationsContext(ctx context.Context, params *slack.GetConversationsParameters) (channels []slack.Channel, nextCursor string, err error) {
	args := m.Called(ctx, params)
	var ok bool
	channels, ok = args.Get(0).([]slack.Channel)
	if !ok {
		m.t.Error("failed to cast channels")
	}
	nextCursor = args.String(1)
	err = args.Error(2)
	return channels, nextCursor, err
}

type mockStorage struct {
	t *testing.T
	mock.Mock
}

func (m *mockStorage) SetChannels(ctx context.Context, channels []slack.Channel) error {
	args := m.Called(ctx, channels)
	return args.Error(0)
}

func (m *mockStorage) GetByChannelName(ctx context.Context, channelName string) (*slack.Channel, error) {
	args := m.Called(ctx, channelName)
	channel, ok := args.Get(0).(*slack.Channel)
	if channel != nil && !ok {
		m.t.Error("failed to cast channel")
	}
	return channel, args.Error(1)
}

func (m *mockStorage) NeedRefresh(ctx context.Context) bool {
	args := m.Called(ctx)
	return args.Bool(0)
}

func TestResolverLookup__UseCache(t *testing.T) {
	client := &mockSlackClient{t: t}
	defer client.AssertExpectations(t)
	storage := &mockStorage{t: t}
	defer storage.AssertExpectations(t)

	storage.On("NeedRefresh", mock.Anything).Return(false).Times(1)
	storage.On("GetByChannelName", mock.Anything, "test").Return(nil, slackcnr.ErrNotFound)
	r := slackcnr.New(client,
		slackcnr.WithCacheStorage(storage),
		slackcnr.WithBatchSize(1),
		slackcnr.WithExcludeArchived(),
	)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_, err := r.Lookup(ctx, "test")
	require.ErrorIs(t, err, slackcnr.ErrNotFound)
}

func TestResolverLookup__NoCache(t *testing.T) {
	client := &mockSlackClient{t: t}
	defer client.AssertExpectations(t)
	storage := &mockStorage{t: t}
	defer storage.AssertExpectations(t)

	storage.On("NeedRefresh", mock.Anything).Return(true).Times(1)
	client.On("GetConversationsForUserContext", mock.Anything, &slack.GetConversationsForUserParameters{
		Cursor:          "",
		Limit:           1,
		ExcludeArchived: true,
	}).Return([]slack.Channel{
		{
			GroupConversation: slack.GroupConversation{
				Conversation: slack.Conversation{
					ID: "C012345678",
				},
				Name: "test",
			},
		},
	}, "test_cusor", nil)
	storage.On("SetChannels", mock.Anything, []slack.Channel{
		{
			GroupConversation: slack.GroupConversation{
				Conversation: slack.Conversation{
					ID: "C012345678",
				},
				Name: "test",
			},
		},
	}).Return(nil)
	client.On("GetConversationsForUserContext", mock.Anything, &slack.GetConversationsForUserParameters{
		Cursor:          "test_cusor",
		Limit:           1,
		ExcludeArchived: true,
	}).Return([]slack.Channel{
		{
			GroupConversation: slack.GroupConversation{
				Conversation: slack.Conversation{
					ID: "C023456789",
				},
				Name: "test2",
			},
		},
	}, "", nil)
	storage.On("SetChannels", mock.Anything, []slack.Channel{
		{
			GroupConversation: slack.GroupConversation{
				Conversation: slack.Conversation{
					ID: "C023456789",
				},
				Name: "test2",
			},
		},
	}).Return(nil)
	storage.On("GetByChannelName", mock.Anything, "test").Return(&slack.Channel{
		GroupConversation: slack.GroupConversation{
			Conversation: slack.Conversation{
				ID: "C012345678",
			},
			Name: "test",
		},
	}, nil)
	r := slackcnr.New(client,
		slackcnr.WithCacheStorage(storage),
		slackcnr.WithBatchSize(1),
		slackcnr.WithExcludeArchived(),
	)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	channel, err := r.Lookup(ctx, "test")
	require.NoError(t, err)
	require.NotNil(t, channel)
	require.Equal(t, "C012345678", channel.ID)
}

func TestResolverLookup__RefreshOnCacheMiss(t *testing.T) {
	client := &mockSlackClient{t: t}
	defer client.AssertExpectations(t)
	storage := &mockStorage{t: t}
	defer storage.AssertExpectations(t)

	storage.On("NeedRefresh", mock.Anything).Return(false).Times(1)
	storage.On("GetByChannelName", mock.Anything, "test").Return(nil, slackcnr.ErrNotFound).Times(1)
	client.On("GetConversationsForUserContext", mock.Anything, &slack.GetConversationsForUserParameters{
		Cursor:          "",
		Limit:           1,
		ExcludeArchived: true,
	}).Return([]slack.Channel{
		{
			GroupConversation: slack.GroupConversation{
				Conversation: slack.Conversation{
					ID: "C012345678",
				},
				Name: "test",
			},
		},
	}, "test_cusor", nil)
	storage.On("SetChannels", mock.Anything, []slack.Channel{
		{
			GroupConversation: slack.GroupConversation{
				Conversation: slack.Conversation{
					ID: "C012345678",
				},
				Name: "test",
			},
		},
	}).Return(nil)
	client.On("GetConversationsForUserContext", mock.Anything, &slack.GetConversationsForUserParameters{
		Cursor:          "test_cusor",
		Limit:           1,
		ExcludeArchived: true,
	}).Return([]slack.Channel{
		{
			GroupConversation: slack.GroupConversation{
				Conversation: slack.Conversation{
					ID: "C023456789",
				},
				Name: "test2",
			},
		},
	}, "", nil)
	storage.On("SetChannels", mock.Anything, []slack.Channel{
		{
			GroupConversation: slack.GroupConversation{
				Conversation: slack.Conversation{
					ID: "C023456789",
				},
				Name: "test2",
			},
		},
	}).Return(nil)
	storage.On("GetByChannelName", mock.Anything, "test").Return(&slack.Channel{
		GroupConversation: slack.GroupConversation{
			Conversation: slack.Conversation{
				ID: "C012345678",
			},
			Name: "test",
		},
	}, nil)
	r := slackcnr.New(client,
		slackcnr.WithCacheStorage(storage),
		slackcnr.WithBatchSize(1),
		slackcnr.WithExcludeArchived(),
		slackcnr.WithRefreshOnCacheMiss(),
	)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	channel, err := r.Lookup(ctx, "test")
	require.NoError(t, err)
	require.NotNil(t, channel)
	require.Equal(t, "C012345678", channel.ID)
}
