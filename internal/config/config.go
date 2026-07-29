package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

// Config представляет структуру конфигурации приложения.
type Config struct {
	Server struct {
		Port                int  `yaml:"port"`
		Debug               bool `yaml:"debug"`
		RecordRetentionDays int  `yaml:"record_retention_days"` // Количество дней для хранения записей (0 - бесконечно)
	} `yaml:"server"`

	Auth struct {
		Username string `yaml:"username"`
		Password string `yaml:"password"`
	} `yaml:"auth"`

	Cameras []CameraConfig `yaml:"cameras"`
}

// CameraConfig описывает настройки подключения для отдельной камеры.
type CameraConfig struct {
	ID            string `yaml:"id" json:"id"`
	URL           string `yaml:"url" json:"url"`
	Record        bool   `yaml:"record" json:"record"`
	RetentionDays int    `yaml:"retention_days" json:"retentionDays"` // 0 означает использование глобального значения
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
