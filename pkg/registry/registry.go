// Package registry defines core service abstraction interfaces (StateStore, Authenticator,
// BlobStore, EventBus) and provides a centralized dependency injection mechanism.
//
// By registering concrete implementations during server startup, subsystems throughout RUSEON Core
// can access database persistence, file storage, security tokens, and event pipelines with loose coupling.
package registry

import (
	"context"
	"io"
	"io/fs"

	"github.com/gin-gonic/gin"

	"github.com/RUSEGAL/ruseon-core/v2/internal/models"
	"github.com/RUSEGAL/ruseon-core/v2/pkg/config"
)

// StateStore defines the persistence interface for application metadata, stream definitions,
// user accounts, tags, and folders.
//
// All implementations must ensure thread-safety and transactional consistency.
type StateStore interface {
	// Ping verifies database connectivity and health.
	Ping(ctx context.Context) error

	// SaveCamera inserts or updates a camera stream configuration.
	SaveCamera(cam *config.CameraConfig) error
	// GetCamera retrieves a camera configuration by its unique ID. Returns ErrNotFound if missing.
	GetCamera(id string) (*config.CameraConfig, error)
	// DeleteCamera removes a camera configuration by ID.
	DeleteCamera(id string) error
	// ListCameras returns all stored camera configurations.
	ListCameras() ([]config.CameraConfig, error)
	// UpdateCameraTx executes updateFn within an atomic transaction for the specified camera ID.
	// If updateFn returns false, the transaction is aborted without modification.
	UpdateCameraTx(id string, updateFn func(cam *config.CameraConfig) bool) error
	// BatchUpdateTraffic atomically accumulates traffic usage bytes for multiple cameras in a single batch.
	BatchUpdateTraffic(updates map[string]uint64, nowMonth string) error

	// SaveTag creates or updates a metadata tag.
	SaveTag(tag *config.TagConfig) error
	// GetTag retrieves a tag by its unique ID. Returns ErrNotFound if missing.
	GetTag(id string) (*config.TagConfig, error)
	// DeleteTag removes a tag by its ID.
	DeleteTag(id string) error
	// ListTags returns all stored metadata tags.
	ListTags() ([]config.TagConfig, error)

	// SaveFolder creates or updates a folder/group.
	SaveFolder(folder *config.FolderConfig) error
	// GetFolder retrieves a folder by its unique ID. Returns ErrNotFound if missing.
	GetFolder(id string) (*config.FolderConfig, error)
	// DeleteFolder removes a folder by its ID.
	DeleteFolder(id string) error
	// ListFolders returns all stored folders.
	ListFolders() ([]config.FolderConfig, error)

	// SaveUser creates or updates a user credential record.
	SaveUser(user *models.User) error
	// GetUser retrieves a user by their username. Returns ErrNotFound if missing.
	GetUser(username string) (*models.User, error)
	// ListUsers returns all user accounts.
	ListUsers() ([]models.User, error)
	// DeleteUser removes a user account by username.
	DeleteUser(username string) error
	// HasUsers reports whether any user accounts exist in the database.
	HasUsers() (bool, error)

	// MigrateFromConfig imports cameras and tags from YAML config into the store on initial launch.
	MigrateFromConfig(cfg *config.Config) error
	// ExportJSON exports all stored entities as a JSON backup payload.
	ExportJSON() ([]byte, error)
	// ImportJSON restores and merges entities from a JSON backup payload.
	ImportJSON(data []byte) error
	// BackupBadger writes a raw incremental/full BadgerDB stream backup to w.
	BackupBadger(w io.Writer) error
	// Sync flushes write-ahead logs and commits dirty pages to disk.
	Sync() error
	// Close safely terminates database operations and releases file locks.
	Close() error
}

// Authenticator defines the security provider interface for authentication,
// API access validation, and stream token issuance.
type Authenticator interface {
	// Login handles the HTTP credentials authentication endpoint.
	Login(c *gin.Context)
	// Middleware returns a Gin middleware enforcing valid Bearer JWT access tokens.
	Middleware() gin.HandlerFunc
	// StreamMiddleware returns a Gin middleware validating short-lived "?token=" query tokens for media streams.
	StreamMiddleware() gin.HandlerFunc
	// GenerateStreamToken generates a short-lived (60s) playback token for the specified cameraID.
	GenerateStreamToken(cameraID string) (string, error)
}

// WriteSeekCloser combines io.Writer, io.Seeker, and io.Closer for file operations.
type WriteSeekCloser interface {
	io.Writer
	io.Seeker
	io.Closer
}

// CacheDropper is an optional interface implemented by files that support OS Page Cache purging.
type CacheDropper interface {
	// DropCache requests the kernel to flush and evict cached file pages from memory.
	DropCache() error
}

// BlobStore defines the binary storage interface for video archives and media chunks.
type BlobStore interface {
	// Write writes a binary blob to the specified path.
	Write(path string, data []byte) error
	// Read reads the full binary blob from the specified path.
	Read(path string) ([]byte, error)
	// Delete deletes the file or directory at the specified path.
	Delete(path string) error
	// Stat returns file metadata information.
	Stat(path string) (fs.FileInfo, error)
	// ReadDir reads the directory entries at the specified path.
	ReadDir(path string) ([]fs.DirEntry, error)
	// Create creates or truncates a file at path for reading/writing.
	Create(path string) (WriteSeekCloser, error)
	// Open opens an existing file at path for reading.
	Open(path string) (io.ReadSeekCloser, error)
	// MkdirAll creates a directory hierarchy along with any necessary parents.
	MkdirAll(path string) error
	// Rename renames or moves a file from oldPath to newPath.
	Rename(oldPath, newPath string) error
}

// EventBus defines the event publishing and worker lifecycle interface.
type EventBus interface {
	// Publish dispatches an asynchronous event for a given topic and cameraID.
	Publish(topic string, cameraID string, data any)
	// Stop gracefully terminates all event bus background workers.
	Stop()
}

// Global registry variables holding active service instances.
var (
	// CurrentStateStore holds the active StateStore singleton.
	CurrentStateStore StateStore
	// CurrentAuthenticator holds the active Authenticator singleton.
	CurrentAuthenticator Authenticator
	// CurrentBlobStore holds the active BlobStore singleton.
	CurrentBlobStore BlobStore
	// CurrentEventBus holds the active EventBus singleton.
	CurrentEventBus EventBus
)

// RegisterStateStore injects the global StateStore instance.
func RegisterStateStore(store StateStore) {
	CurrentStateStore = store
}

// RegisterAuthenticator injects the global Authenticator instance.
func RegisterAuthenticator(auth Authenticator) {
	CurrentAuthenticator = auth
}

// RegisterBlobStore injects the global BlobStore instance.
func RegisterBlobStore(blob BlobStore) {
	CurrentBlobStore = blob
}

// RegisterEventBus injects the global EventBus instance.
func RegisterEventBus(bus EventBus) {
	CurrentEventBus = bus
}
