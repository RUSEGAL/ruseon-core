package mqtt

import (
	"context"
	"encoding/json"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/rs/zerolog/log"

	"github.com/RUSEGAL/ruseon-core/internal/buffer"
	"github.com/RUSEGAL/ruseon-core/pkg/config"
	"github.com/RUSEGAL/ruseon-core/pkg/grpc/pb"
)

// Publisher handles publishing AI metadata to an MQTT broker.
type Publisher struct {
	client mqtt.Client
	config config.MQTTConfig
	queue  *buffer.LockFreeRingBuffer[pb.MetadataRequest]
	cancel context.CancelFunc
}

// NewPublisher creates and starts a new MQTT publisher if enabled.
func NewPublisher(cfg config.MQTTConfig) (*Publisher, error) {
	if !cfg.Enabled {
		return nil, nil
	}

	opts := mqtt.NewClientOptions()
	opts.AddBroker(cfg.Broker)
	if cfg.Username != "" {
		opts.SetUsername(cfg.Username)
		opts.SetPassword(cfg.Password)
	}
	opts.SetClientID("ruseon-core")
	opts.SetAutoReconnect(true)
	opts.SetConnectRetry(true)
	opts.SetConnectRetryInterval(5 * time.Second)
	opts.SetConnectTimeout(3 * time.Second)

	client := mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		return nil, token.Error()
	}

	log.Info().Str("broker", cfg.Broker).Msg("Connected to MQTT broker")

	ctx, cancel := context.WithCancel(context.Background())
	pub := &Publisher{
		client: client,
		config: cfg,
		queue:  buffer.NewLockFreeRingBuffer[pb.MetadataRequest](1024), // capacity 1024
		cancel: cancel,
	}

	go pub.worker(ctx)

	return pub, nil
}

// Push adds metadata to the lock-free queue for asynchronous publishing.
func (p *Publisher) Push(meta *pb.MetadataRequest) {
	if p == nil {
		return
	}
	p.queue.Push(meta)
}

// Close stops the worker and disconnects the client.
func (p *Publisher) Close() {
	if p == nil {
		return
	}
	p.cancel()
	p.client.Disconnect(250)
}

func (p *Publisher) worker(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Millisecond) // Poll queue
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Drain queue
			for {
				item := p.queue.Pop()
				if item == nil {
					break
				}
				
				// Publish to MQTT
				payload, err := json.Marshal(item)
				if err != nil {
					log.Error().Err(err).Msg("Failed to marshal metadata for MQTT")
					continue
				}

				// QoS 0, not retained
				token := p.client.Publish(p.config.Topic, 0, false, payload)
				// We don't wait for token to avoid blocking, it's fire and forget
				go func(t mqtt.Token) {
					if t.Wait() && t.Error() != nil {
						log.Debug().Err(t.Error()).Msg("Failed to publish to MQTT")
					}
				}(token)
			}
		}
	}
}
