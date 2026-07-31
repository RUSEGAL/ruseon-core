package config

import (
	"crypto/rand"
	"encoding/hex"
	"os"

	"gopkg.in/yaml.v3"
)

// Config представляет структуру конфигурации приложения.
type Config struct {
	Server struct {
		Port                int  `yaml:"port"`
		Debug               bool `yaml:"debug"`
		RecordRetentionDays int  `yaml:"record_retention_days"` // Количество дней для хранения записей (0 - бесконечно)
		GCPercent           int  `yaml:"gc_percent"`            // Тюнинг GOGC (по умолчанию 50)
		GCMemoryLimitMB     int  `yaml:"gc_memory_limit_mb"`    // Тюнинг GOMEMLIMIT (в мегабайтах)
	} `yaml:"server"`

	Auth struct {
		Username string `yaml:"username"`
		Password string `yaml:"password"`
		Secret   string `yaml:"secret,omitempty"` // Секретный ключ для JWT
	} `yaml:"auth"`

	GlobalTags []TagConfig    `yaml:"global_tags,omitempty"`
	Cameras    []CameraConfig `yaml:"cameras,omitempty"`
}

// TagConfig описывает пользовательскую метку (тег)
type TagConfig struct {
	ID    string `yaml:"id" json:"id"`
	Name  string `yaml:"name" json:"name"`
	Color string `yaml:"color" json:"color"`
}

// DisableRecord описывает событие изменения статуса отключения.
type DisableRecord struct {
	Timestamp string `yaml:"timestamp" json:"timestamp"`
	Action    string `yaml:"action" json:"action"` // "disable" или "enable"
	Reason    string `yaml:"reason,omitempty" json:"reason"`
}

// CameraConfig описывает настройки подключения для отдельной камеры.
type CameraConfig struct {
	ID            string   `yaml:"id" json:"id"`
	URL           string   `yaml:"url" json:"url"`
	Record        bool     `yaml:"record" json:"record"`
	RetentionDays int      `yaml:"retention_days" json:"retentionDays"` // 0 означает использование глобального значения
	Tags          []string `yaml:"tags,omitempty" json:"tags"`
	Comment       string   `yaml:"comment,omitempty" json:"comment"`
	SimPhone      string   `yaml:"sim_phone,omitempty" json:"simPhone"`
	SimICCID      string   `yaml:"sim_iccid,omitempty" json:"simICCID"`

	// Биллинг и трафик
	TrafficLimit   uint64 `yaml:"traffic_limit" json:"trafficLimit"`     // Лимит в байтах
	TrafficUsed    uint64 `yaml:"traffic_used" json:"trafficUsed"`       // Использовано в байтах
	LastResetMonth string `yaml:"last_reset_month" json:"lastResetMonth"` // Месяц последнего сброса (YYYY-MM)

	// Отключение
	Disabled       bool            `yaml:"disabled" json:"disabled"`
	DisableReason  string          `yaml:"disable_reason,omitempty" json:"disableReason"`
	DisableHistory []DisableRecord `yaml:"disable_history,omitempty" json:"disableHistory"`
	RecordHistory  []DisableRecord `yaml:"record_history,omitempty" json:"recordHistory"`
}

// Load считывает конфигурацию из файла.
func Load(path string) (*Config, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var cfg Config
	decoder := yaml.NewDecoder(file)
	if err := decoder.Decode(&cfg); err != nil {
		return nil, err
	}

	// Генерируем JWT Secret, если его нет
	if cfg.Auth.Secret == "" {
		bytes := make([]byte, 32)
		if _, err := rand.Read(bytes); err == nil {
			cfg.Auth.Secret = hex.EncodeToString(bytes)
			cfg.Save(path)
		}
	}

	// Устанавливаем дефолты для GC (если не заданы)
	if cfg.Server.GCPercent == 0 {
		cfg.Server.GCPercent = 50
	}

	return &cfg, nil
}

// Save сохраняет текущую конфигурацию в файл.
func (c *Config) Save(path string) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := yaml.NewEncoder(file)
	defer encoder.Close()
	return encoder.Encode(c)
}
