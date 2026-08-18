package mqtt

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/RUSEGAL/ruseon-core/pkg/config"
	"github.com/RUSEGAL/ruseon-core/pkg/grpc/pb"
)

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
