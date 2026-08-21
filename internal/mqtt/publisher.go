package mqtt

import (
	"context"
	"encoding/json"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/rs/zerolog/log"

	"github.com/RUSEGAL/ruseon-core/pkg/config"
	"github.com/RUSEGAL/ruseon-core/pkg/grpc/pb"
)

// Publisher handles publishing AI metadata to an MQTT broker.
type Publisher struct {
	client    mqtt.Client
	config    config.MQTTConfig
	queue     chan *pb.MetadataRequest
	tokenChan chan mqtt.Token
	cancel    context.CancelFunc
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
		client:    client,
		config:    cfg,
		queue:     make(chan *pb.MetadataRequest, 1024),
		tokenChan: make(chan mqtt.Token, 256),
		cancel:    cancel,
	}

	go pub.worker(ctx)
	go pub.tokenWaiter(ctx)

	return pub, nil
}

// Push adds metadata to the channel queue for asynchronous publishing.
func (p *Publisher) Push(meta *pb.MetadataRequest) {
	if p == nil || meta == nil {
		return
	}
	select {
	case p.queue <- meta:
	default:
		// Queue full, drop newest to avoid blocking producer
	}
}

// Close stops the worker and disconnects the client.
func (p *Publisher) Close() {
	if p == nil {
		return
	}
	p.cancel()
	p.client.Disconnect(250)
}

func (p *Publisher) tokenWaiter(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case token, ok := <-p.tokenChan:
			if !ok {
				return
			}
			if token != nil && token.Wait() && token.Error() != nil {
				log.Debug().Err(token.Error()).Msg("Failed to publish to MQTT")
			}
		}
	}
}

func (p *Publisher) worker(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case item := <-p.queue:
			if item == nil {
				continue
			}

			// Publish to MQTT
			payload, err := json.Marshal(item)
			if err != nil {
				log.Error().Err(err).Msg("Failed to marshal metadata for MQTT")
				continue
			}

			// QoS 0, not retained
			token := p.client.Publish(p.config.Topic, 0, false, payload)
			if token != nil && p.tokenChan != nil {
				select {
				case p.tokenChan <- token:
				default:
					// Token queue is full, skip waiting to keep publishing non-blocking
				}
			}
		}
	}
}

