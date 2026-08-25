// Package config provides data structures, validation, and serialization
// mechanisms for loading and persisting RUSEON Core application settings in YAML format.
package config

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config represents the complete application configuration hierarchy.
//
// It contains network server options, TLS certificates, Go runtime tuning,
// WebRTC/HLS/gRPC protocol settings, authentication secrets, dynamic camera streams,
// metadata tags, and asynchronous event notifications.
type Config struct {
	// Server groups HTTP, streaming, RPC, and runtime configuration settings.
	Server struct {
		// Port is the primary HTTP/REST API listening port (e.g. 8080).
		Port int `yaml:"port"`
		// Debug enables verbose debug logging and runtime diagnostics (pprof, statsviz).
		Debug bool `yaml:"debug"`
		// PprofPort sets the dedicated localhost listening port for the standard pprof profiler (0 to disable).
		PprofPort int `yaml:"pprof_port"`
		// RecordRetentionDays specifies the global default retention duration for MP4 archive recordings (0 for infinite).
		RecordRetentionDays int `yaml:"record_retention_days"`
		// GCPercent configures the Go runtime garbage collection target percentage (default: 50).
		GCPercent int `yaml:"gc_percent"`
		// GCMemoryLimitMB configures the Go runtime soft memory limit via GOMEMLIMIT (in megabytes).
		GCMemoryLimitMB int `yaml:"gc_memory_limit_mb"`
		// CORSAllowedOrigins lists allowed origin strings for Cross-Origin Resource Sharing (CORS).
		CORSAllowedOrigins []string `yaml:"cors_allowed_origins" json:"corsAllowedOrigins"`

		// HLS specifies settings for HTTP Live Streaming delivery.
		HLS struct {
			// LiveSegmentsInMemory defines the sliding window size of TS segments kept in memory (minimum 3 per RFC 8216).
			LiveSegmentsInMemory int `yaml:"live_segments_in_memory" json:"liveSegmentsInMemory"`
		} `yaml:"hls" json:"hls"`

		// GRPC specifies settings for the gRPC frame extractor and AI metadata server.
		GRPC struct {
			// Address is the network interface address to bind (e.g. "0.0.0.0" or "127.0.0.1").
			Address string `yaml:"address" json:"address"`
			// Port is the gRPC TCP listening port (default: 50051).
			Port int `yaml:"port" json:"port"`
		} `yaml:"grpc" json:"grpc"`

		// TLS specifies TLS certificate paths for secure HTTPS and gRPC communication.
		TLS struct {
			// CertFile is the filesystem path to the public PEM-encoded certificate.
			CertFile string `yaml:"cert_file" json:"certFile"`
			// KeyFile is the filesystem path to the private PEM-encoded key.
			KeyFile string `yaml:"key_file" json:"keyFile"`
		} `yaml:"tls" json:"tls"`

		// WebRTC specifies network, ICE, and NAT traversal parameters for WebRTC streaming.
		WebRTC struct {
			// ListenPort is the UDP port for single-port ICE UDP muxing (e.g. 8555; 0 for ephemeral).
			ListenPort int `yaml:"listen_port" json:"listenPort"`
			// NAT1To1IPs provides public IP addresses mapped 1:1 to the server for ICE host candidates behind NAT.
			NAT1To1IPs []string `yaml:"nat_1_to_1_ips" json:"nat1To1IPs"`
			// ICEServers lists STUN/TURN server URLs (e.g. "stun:stun.l.google.com:19302").
			ICEServers []string `yaml:"ice_servers" json:"iceServers"`
			// ICETransportPolicy sets the candidate filtering policy ("all" or "relay").
			ICETransportPolicy string `yaml:"ice_transport_policy" json:"iceTransportPolicy"`
			// TURNUsername is the authentication username for TURN relay servers.
			TURNUsername string `yaml:"turn_username" json:"turnUsername"`
			// TURNPassword is the authentication credential for TURN relay servers.
			TURNPassword string `yaml:"turn_password" json:"turnPassword"`
		} `yaml:"webrtc" json:"webrtc"`
	} `yaml:"server"`

	// Auth contains security and token configuration.
	Auth struct {
		// Secret is the HMAC-SHA256 secret key used to sign and verify JWT authentication tokens.
		Secret string `yaml:"secret,omitempty"`
	} `yaml:"auth"`

	// GlobalTags defines available user metadata tags for organizing cameras.
	GlobalTags []TagConfig `yaml:"global_tags,omitempty"`
	// Cameras defines the initial camera list (migrated to BadgerDB on first start).
	Cameras []CameraConfig `yaml:"cameras,omitempty"`
	// Events defines notification sinks (Webhooks and MQTT).
	Events EventsConfig `yaml:"events,omitempty"`
}

// WebhookConfig specifies the configuration for outbound HTTP webhook notifications.
type WebhookConfig struct {
	// URL is the destination HTTP/HTTPS endpoint.
	URL string `yaml:"url" json:"url"`
	// Topics filters which event topics trigger this webhook (empty or ["*"] for all).
	Topics []string `yaml:"topics,omitempty" json:"topics"`
	// Secret is an optional HMAC-SHA256 signing secret sent in the "X-Signature" header.
	Secret string `yaml:"secret,omitempty" json:"secret"`
}

// MQTTConfig specifies the connection parameters for publishing events and AI metadata to an MQTT broker.
type MQTTConfig struct {
	// Enabled indicates whether the MQTT publisher worker is activated.
	Enabled bool `yaml:"enabled" json:"enabled"`
	// Broker is the MQTT broker URI (e.g. "tcp://localhost:1883").
	Broker string `yaml:"broker" json:"broker"`
	// Username is the optional client username for broker authentication.
	Username string `yaml:"username,omitempty" json:"username"`
	// Password is the optional client password for broker authentication.
	Password string `yaml:"password,omitempty" json:"password"`
	// Topic is the base MQTT topic to publish metadata to (e.g. "ruseon/metadata").
	Topic string `yaml:"topic" json:"topic"`
}

// EventsConfig aggregates all outbound event delivery configuration sinks.
type EventsConfig struct {
	// Webhooks lists configured HTTP webhook endpoints.
	Webhooks []WebhookConfig `yaml:"webhooks,omitempty" json:"webhooks"`
	// MQTT contains settings for MQTT event publishing.
	MQTT MQTTConfig `yaml:"mqtt,omitempty" json:"mqtt"`
}

// TagConfig represents a custom metadata label for categorizing cameras.
type TagConfig struct {
	// ID is the unique identifier of the tag.
	ID string `yaml:"id" json:"id"`
	// Name is the human-readable display name.
	Name string `yaml:"name" json:"name"`
	// Color is a hex color code (e.g. "#FF5722") for UI rendering.
	Color string `yaml:"color" json:"color"`
}

// FolderConfig represents a logical folder or group for hierarchical camera organization.
type FolderConfig struct {
	// ID is the unique identifier of the folder.
	ID string `yaml:"id" json:"id"`
	// Name is the folder display name.
	Name string `yaml:"name" json:"name"`
}

// DisableRecord records an audit event tracking when a camera was enabled or disabled.
type DisableRecord struct {
	// Timestamp is the RFC3339 formatted time of the status change.
	Timestamp string `yaml:"timestamp" json:"timestamp"`
	// Action indicates the transition ("disable" or "enable").
	Action string `yaml:"action" json:"action"`
	// Reason provides an optional explanation (e.g. "maintenance", "technical", "payment").
	Reason string `yaml:"reason,omitempty" json:"reason"`
}

// CameraConfig describes the configuration and operational parameters of an individual camera stream.
type CameraConfig struct {
	// ID is the unique identifier for the camera (used in API and stream URLs).
	ID string `yaml:"id" json:"id"`
	// URL is the RTSP source stream URI (e.g. "rtsp://user:pass@192.168.1.100:554/live").
	URL string `yaml:"url" json:"url"`
	// Record enables continuous local fragmented MP4 (fMP4) archive recording.
	Record bool `yaml:"record" json:"record"`
	// RetentionDays specifies custom archive retention in days (0 uses global Server.RecordRetentionDays).
	RetentionDays int `yaml:"retention_days" json:"retentionDays"`
	// Tags contains IDs of associated TagConfig labels.
	Tags []string `yaml:"tags,omitempty" json:"tags"`
	// FolderID is the ID of the parent FolderConfig group.
	FolderID string `yaml:"folder_id,omitempty" json:"folderId"`
	// Comment is an optional operator note.
	Comment string `yaml:"comment,omitempty" json:"comment"`
	// SimPhone is the cellular phone number if the camera connects via 4G/5G modem.
	SimPhone string `yaml:"sim_phone,omitempty" json:"simPhone"`
	// SimICCID is the SIM card integrated circuit card identifier.
	SimICCID string `yaml:"sim_iccid,omitempty" json:"simICCID"`
	// LazyHLS activates on-demand HLS muxing only when active viewers are connected.
	LazyHLS bool `yaml:"lazy_hls,omitempty" json:"lazyHLS"`
	// Transport sets the preferred RTSP transport ("tcp", "udp", or "auto").
	Transport string `yaml:"transport,omitempty" json:"transport"`
	// TokenAuth requires short-lived stream tokens for video playback endpoints.
	TokenAuth bool `yaml:"token_auth,omitempty" json:"tokenAuth"`

	// Billing and traffic tracking
	// TrafficLimit is the monthly traffic quota in bytes (0 for unlimited / default 200GB).
	TrafficLimit uint64 `yaml:"traffic_limit" json:"trafficLimit"`
	// TrafficUsed is the accumulated monthly traffic consumption in bytes.
	TrafficUsed uint64 `yaml:"traffic_used" json:"trafficUsed"`
	// LastResetMonth is the last billing cycle reset period formatted as "YYYY-MM".
	LastResetMonth string `yaml:"last_reset_month" json:"lastResetMonth"`

	// Stream status and disable history
	// Disabled indicates whether stream ingest is paused.
	Disabled bool `yaml:"disabled" json:"disabled"`
	// DisableReason describes why the stream was disabled.
	DisableReason string `yaml:"disable_reason,omitempty" json:"disableReason"`
	// DisableHistory tracks past disable/enable lifecycle events.
	DisableHistory []DisableRecord `yaml:"disable_history,omitempty" json:"disableHistory"`
	// RecordHistory tracks changes to the recording state.
	RecordHistory []DisableRecord `yaml:"record_history,omitempty" json:"recordHistory"`
}

// Clone returns a deep copy of the CameraConfig, ensuring slice fields (Tags, DisableHistory, RecordHistory)
// are safely duplicated to prevent data races during concurrent read/write operations.
func (c CameraConfig) Clone() CameraConfig {
	out := c
	if c.Tags != nil {
		out.Tags = make([]string, len(c.Tags))
		copy(out.Tags, c.Tags)
	}
	if c.DisableHistory != nil {
		out.DisableHistory = make([]DisableRecord, len(c.DisableHistory))
		copy(out.DisableHistory, c.DisableHistory)
	}
	if c.RecordHistory != nil {
		out.RecordHistory = make([]DisableRecord, len(c.RecordHistory))
		copy(out.RecordHistory, c.RecordHistory)
	}
	return out
}

// Load reads and parses a YAML configuration file from the specified path.
//
// It performs initialization steps:
//   - Generates and persists a secure random 32-byte JWT secret if not configured.
//   - Applies defaults for GCPercent (50), HLS LiveSegmentsInMemory (3), and gRPC Port (50051).
//
// Returns a pointer to the populated Config or an error if file reading or decoding fails.
func Load(path string) (*Config, error) {
	file, err := os.Open(filepath.Clean(path)) // #nosec G304
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var cfg Config
	decoder := yaml.NewDecoder(file)
	if err := decoder.Decode(&cfg); err != nil {
		return nil, err
	}

	// Generate random JWT Secret if empty
	if cfg.Auth.Secret == "" {
		bytes := make([]byte, 32)
		if _, err := rand.Read(bytes); err != nil {
			return nil, fmt.Errorf("generate random JWT secret: %w", err)
		}
		cfg.Auth.Secret = hex.EncodeToString(bytes)
		if err := cfg.Save(path); err != nil {
			return nil, fmt.Errorf("persist generated JWT secret: %w", err)
		}
	}

	// Set defaults for GC
	if cfg.Server.GCPercent == 0 {
		cfg.Server.GCPercent = 50
	}

	// Set defaults for HLS (minimum 3 per RFC 8216)
	if cfg.Server.HLS.LiveSegmentsInMemory < 3 {
		cfg.Server.HLS.LiveSegmentsInMemory = 3
	}

	// Set defaults for gRPC
	if cfg.Server.GRPC.Port == 0 {
		cfg.Server.GRPC.Port = 50051
	}

	return &cfg, nil
}

// Save serializes and writes the configuration to the specified file path in YAML format
// with restricted file permissions (0600) to protect authentication secrets.
func (c *Config) Save(path string) error {
	cleanPath := filepath.Clean(path)
	file, err := os.OpenFile(cleanPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600) // #nosec G304
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := yaml.NewEncoder(file)
	defer encoder.Close()
	if err := encoder.Encode(c); err != nil {
		return err
	}
	_ = os.Chmod(cleanPath, 0600)
	return nil
}
