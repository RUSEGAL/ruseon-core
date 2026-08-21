package mqtt

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/RUSEGAL/ruseon-core/pkg/config"
	"github.com/RUSEGAL/ruseon-core/pkg/grpc/pb"
)

type mockToken struct {
	mqtt.Token
	err error
}

func (m *mockToken) Wait() bool {
	return true
}

func (m *mockToken) Error() error {
	return m.err
}

type mockMQTTClient struct {
	mqtt.Client
	mu           sync.Mutex
	published    [][]byte
	tokenErr     error
	disconnected bool
}

func (m *mockMQTTClient) Publish(_ string, _ byte, _ bool, payload interface{}) mqtt.Token {
	m.mu.Lock()
	defer m.mu.Unlock()
	if b, ok := payload.([]byte); ok {
		m.published = append(m.published, b)
	}
	return &mockToken{err: m.tokenErr}
}

func (m *mockMQTTClient) Disconnect(_ uint) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.disconnected = true
}

func TestPublisher_Disabled(t *testing.T) {
	pub, err := NewPublisher(config.MQTTConfig{Enabled: false})
	assert.NoError(t, err)
	assert.Nil(t, pub)

	// Verify nil safety
	pub.Push(&pb.MetadataRequest{CameraId: "cam1"})
	pub.Close()
}

func TestPublisher_InvalidBroker(t *testing.T) {
	pub, err := NewPublisher(config.MQTTConfig{
		Enabled:  true,
		Broker:   "://invalid-url-scheme",
		Username: "user",
		Password: "password",
	})
	assert.Error(t, err)
	assert.Nil(t, pub)
}

func TestPublisher_Worker_PublishAndDrain(t *testing.T) {
	mockClient := &mockMQTTClient{}
	ctx, cancel := context.WithCancel(context.Background())

	pub := &Publisher{
		client: mockClient,
		config: config.MQTTConfig{
			Enabled: true,
			Topic:   "ruseon/metadata",
		},
		queue:     make(chan *pb.MetadataRequest, 1024),
		tokenChan: make(chan mqtt.Token, 256),
		cancel:    cancel,
	}

	go pub.worker(ctx)
	go pub.tokenWaiter(ctx)

	// Push 3 metadata requests
	pub.Push(&pb.MetadataRequest{CameraId: "cam-1", Pts: 100})
	pub.Push(&pb.MetadataRequest{CameraId: "cam-2", Pts: 200})
	pub.Push(&pb.MetadataRequest{CameraId: "cam-3", Pts: 300})

	// Wait for worker ticker to drain
	require.Eventually(t, func() bool {
		mockClient.mu.Lock()
		defer mockClient.mu.Unlock()
		return len(mockClient.published) == 3
	}, 1*time.Second, 10*time.Millisecond)

	// Validate payloads
	mockClient.mu.Lock()
	var req pb.MetadataRequest
	err := json.Unmarshal(mockClient.published[0], &req)
	require.NoError(t, err)
	assert.Equal(t, "cam-1", req.CameraId)
	assert.Equal(t, int64(100), req.Pts)
	mockClient.mu.Unlock()

	// Close publisher
	pub.Close()
	mockClient.mu.Lock()
	assert.True(t, mockClient.disconnected)
	mockClient.mu.Unlock()
}

func TestPublisher_Worker_TokenError(t *testing.T) {
	mockClient := &mockMQTTClient{
		tokenErr: errors.New("broker connection refused"),
	}
	ctx, cancel := context.WithCancel(context.Background())

	pub := &Publisher{
		client: mockClient,
		config: config.MQTTConfig{
			Enabled: true,
			Topic:   "ruseon/metadata",
		},
		queue:     make(chan *pb.MetadataRequest, 1024),
		tokenChan: make(chan mqtt.Token, 256),
		cancel:    cancel,
	}

	go pub.worker(ctx)
	go pub.tokenWaiter(ctx)

	pub.Push(&pb.MetadataRequest{CameraId: "cam-err", Pts: 500})

	require.Eventually(t, func() bool {
		mockClient.mu.Lock()
		defer mockClient.mu.Unlock()
		return len(mockClient.published) == 1
	}, 1*time.Second, 10*time.Millisecond)

	pub.Close()
}

func TestPublisher_QueueFull(t *testing.T) {
	pub := &Publisher{
		queue: make(chan *pb.MetadataRequest, 2),
	}

	// Fill queue
	pub.Push(&pb.MetadataRequest{CameraId: "cam-1"})
	pub.Push(&pb.MetadataRequest{CameraId: "cam-2"})

	// Overflow push should not block and drop gracefully
	pub.Push(&pb.MetadataRequest{CameraId: "cam-3"})

	assert.Equal(t, 2, len(pub.queue))
}

func TestPublisher_TokenChanFull_And_NilItem(t *testing.T) {
	mockClient := &mockMQTTClient{}
	ctx, cancel := context.WithCancel(context.Background())

	pub := &Publisher{
		client: mockClient,
		config: config.MQTTConfig{
			Enabled: true,
			Topic:   "ruseon/metadata",
		},
		queue:     make(chan *pb.MetadataRequest, 10),
		tokenChan: make(chan mqtt.Token, 1),
		cancel:    cancel,
	}

	// Fill tokenChan
	pub.tokenChan <- &mockToken{}

	go pub.worker(ctx)

	// Send nil item (should be skipped) and a valid item (tokenChan is full, should not block)
	pub.queue <- nil
	pub.queue <- &pb.MetadataRequest{CameraId: "cam-full", Pts: 999}

	require.Eventually(t, func() bool {
		mockClient.mu.Lock()
		defer mockClient.mu.Unlock()
		return len(mockClient.published) == 1
	}, 1*time.Second, 10*time.Millisecond)

	cancel()
}

func TestPublisher_TokenWaiter_ClosedChan(_ *testing.T) {
	pub := &Publisher{
		tokenChan: make(chan mqtt.Token),
	}
	close(pub.tokenChan)

	ctx := context.Background()
	// Should return immediately on closed channel
	pub.tokenWaiter(ctx)
}

