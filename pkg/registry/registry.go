package registry

import (
	"io"
	"io/fs"

	"github.com/gin-gonic/gin"

	"github.com/RUSEGAL/ruseon-core/internal/models"
	"github.com/RUSEGAL/ruseon-core/pkg/config"
)

// StateStore абстрагирует хранение состояния (конфиги камер, тегов и папок).
type StateStore interface {
	SaveCamera(cam *config.CameraConfig) error
	GetCamera(id string) (*config.CameraConfig, error)
	DeleteCamera(id string) error
	ListCameras() ([]config.CameraConfig, error)
	UpdateCameraTx(id string, updateFn func(cam *config.CameraConfig) bool) error

	SaveTag(tag *config.TagConfig) error
	GetTag(id string) (*config.TagConfig, error)
	DeleteTag(id string) error
	ListTags() ([]config.TagConfig, error)

	SaveFolder(folder *config.FolderConfig) error
	GetFolder(id string) (*config.FolderConfig, error)
	DeleteFolder(id string) error
	ListFolders() ([]config.FolderConfig, error)

	SaveUser(user *models.User) error
	GetUser(username string) (*models.User, error)
	HasUsers() (bool, error)

	MigrateFromConfig(cfg *config.Config) error
	ExportJSON() ([]byte, error)
	ImportJSON(data []byte) error
	BackupBadger(w io.Writer) error // В будущем переименуем в Backup(), но пока оставим для совместимости
	Close() error
}

// Authenticator абстрагирует процесс авторизации (Local JWT vs Enterprise OIDC).
type Authenticator interface {
	Login(c *gin.Context)
	Middleware() gin.HandlerFunc
	StreamMiddleware() gin.HandlerFunc
	GenerateStreamToken(cameraID string) (string, error)
}

// WriteSeekCloser combines io.Writer, io.Seeker and io.Closer
type WriteSeekCloser interface {
	io.Writer
	io.Seeker
	io.Closer
}

// CacheDropper позволяет сбросить дисковый кэш ОС (Page Cache) для файла.
type CacheDropper interface {
	DropCache() error
}

// BlobStore абстрагирует запись файлов (LocalFS vs S3).
type BlobStore interface {
	Write(path string, data []byte) error
	Read(path string) ([]byte, error)
	Delete(path string) error
	Stat(path string) (fs.FileInfo, error)
	ReadDir(path string) ([]fs.DirEntry, error)
	Create(path string) (WriteSeekCloser, error)
	Open(path string) (io.ReadSeekCloser, error)
	MkdirAll(path string) error
	Rename(oldPath, newPath string) error
}

// Глобальный реестр (Registry)
var (
	CurrentStateStore    StateStore
	CurrentAuthenticator Authenticator
	CurrentBlobStore     BlobStore
	CurrentEventBus      EventBus
)

// EventBus абстрагирует шину событий
type EventBus interface {
	Publish(topic string, cameraID string, data any)
	Stop()
}

// Функции для инжекции
func RegisterStateStore(store StateStore) {
	CurrentStateStore = store
}

func RegisterAuthenticator(auth Authenticator) {
	CurrentAuthenticator = auth
}

func RegisterBlobStore(blob BlobStore) {
	CurrentBlobStore = blob
}

func RegisterEventBus(bus EventBus) {
	CurrentEventBus = bus
}
