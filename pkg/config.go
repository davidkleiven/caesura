package pkg

import (
	"fmt"
	"io"
	"os"
	"time"

	"gopkg.in/yaml.v2"
)

type LocalFSStoreConfig struct {
	Directory string `yaml:"directory"`
	Database  string `yaml:"database"`
}

type Config struct {
	StoreType string             `yaml:"store_type"`
	LocalFS   LocalFSStoreConfig `yaml:"local_fs"`
	Timeout   time.Duration      `yaml:"timeout"`
}

func (c *Config) Validate() error {
	switch c.StoreType {
	case "in-memory":
		// No additional validation needed for in-memory store
	case "local-fs":
		if c.LocalFS.Directory == "" {
			return fmt.Errorf("local_fs.directory must be specified for local-fs store")
		}
	default:
		return fmt.Errorf("unknown store_type: %s", c.StoreType)
	}

	return nil
}

func NewDefaultConfig() *Config {
	return &Config{StoreType: "in-memory", Timeout: 10 * time.Second}
}

func OverrideFromFile(filePath string, config *Config) (*Config, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return config, fmt.Errorf("error opening config file %s: %w", filePath, err)
	}
	defer f.Close()

	content, err := io.ReadAll(f)
	if err != nil {
		return config, fmt.Errorf("error reading config file %s: %w", filePath, err)
	}

	if err := yaml.Unmarshal(content, config); err != nil {
		return config, fmt.Errorf("error parsing config file %s: %w", filePath, err)
	}
	return config, nil
}

func GetStore(config *Config) BlobStore {
	switch config.StoreType {
	default:
		return NewInMemoryStore()
	}
}
