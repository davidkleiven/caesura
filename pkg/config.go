package pkg

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"time"

	"github.com/davidkleiven/caesura/utils"
	"gopkg.in/yaml.v2"
)

type LocalFSStoreConfig struct {
	Directory string `yaml:"directory"`
	Database  string `yaml:"database"`
}

type Config struct {
	StoreType   string             `yaml:"store_type" env:"CAESURA_STORE_TYPE"`
	LocalFS     LocalFSStoreConfig `yaml:"local_fs"`
	Timeout     time.Duration      `yaml:"timeout" env:"CAESURA_TIMEOUT"`
	Port        int                `yaml:"port" env:"CAESURA_PORT"`
	SecretsPath string             `yaml:"secrets_path" env:"CAESURA_SECRETS_PATH"`
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
	return &Config{StoreType: "in-memory", Timeout: 10 * time.Second, Port: 8080}
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

type EnvGetter func(key string) (string, bool)

// OverrideFromEnv asks all getters in the passed
func OverrideFromEnv(config *Config, getter EnvGetter) *Config {
	t := reflect.TypeOf(config).Elem()
	v := reflect.ValueOf(config).Elem()

	numLoaded := 0
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		fieldValue := v.Field(i)
		envTag := field.Tag.Get("env")
		if envTag == "" || !fieldValue.CanSet() {
			continue
		}

		value, ok := getter(envTag)
		if !ok {
			continue
		}

		switch fieldValue.Kind() {
		case reflect.String:
			fieldValue.SetString(value)
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			intVal := utils.Must(strconv.ParseInt(value, 10, 64))
			fieldValue.SetInt(intVal)
		}
		numLoaded++
	}

	slog.Info("Loaded variables from environment", "num", numLoaded)
	return config
}

func FileEnvGetter(path string) EnvGetter {
	return func(key string) (string, bool) {
		f, err := os.Open(filepath.Join(path, key))
		if err != nil {
			return "", false
		}
		defer f.Close()
		value, err := io.ReadAll(f)
		if err != nil {
			return "", false
		}
		return string(value), true
	}
}

func LoadConfig(configFile string) (*Config, error) {
	config := NewDefaultConfig()
	if configFile != "" {
		if _, err := OverrideFromFile(configFile, config); err != nil {
			return config, err
		}
	}
	OverrideFromEnv(config, os.LookupEnv)
	return OverrideFromEnv(config, FileEnvGetter(config.SecretsPath)), nil
}

func GetStore(config *Config) BlobStore {
	switch config.StoreType {
	default:
		return NewInMemoryStore()
	}
}
