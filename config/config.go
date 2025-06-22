package config

import (
	"io"
	"os"
	"strconv"

	"gopkg.in/yaml.v2"
)

type Config struct {
	Port             string                 `yaml:"port"`
	MaxRequestSizeMB int                    `yaml:"max_request_size_mb"`
	AdaptiveGaussian AdaptiveGaussianConfig `yaml:"adaptive_gaussian"`
}

type AdaptiveGaussianConfig struct {
	BlockSize      int `yaml:"block_size"`
	ThresholdShift int `yaml:"threshold_shift"`
}

func NewDefaultConfig() *Config {
	return &Config{
		Port:             ":8080",
		MaxRequestSizeMB: 10,
		AdaptiveGaussian: AdaptiveGaussianConfig{
			BlockSize:      15,
			ThresholdShift: 2,
		},
	}
}

func UpdateFromReader(config *Config, r io.Reader) error {
	return yaml.NewDecoder(r).Decode(config)
}

func UpdateFromEnv(config *Config) {
	if port, ok := os.LookupEnv("PORT"); ok {
		config.Port = ":" + port
	}
	if maxSize, ok := os.LookupEnv("MAX_REQUEST_SIZE"); ok {
		if size, err := strconv.Atoi(maxSize); err == nil {
			config.MaxRequestSizeMB = size
		}
	}
	if blockSize, ok := os.LookupEnv("ADAPTIVE_GAUSSIAN_BLOCK_SIZE"); ok {
		if size, err := strconv.Atoi(blockSize); err == nil {
			config.AdaptiveGaussian.BlockSize = size
		}
	}
	if thresholdShift, ok := os.LookupEnv("ADAPTIVE_GAUSSIAN_THRESHOLD_SHIFT"); ok {
		if shift, err := strconv.Atoi(thresholdShift); err == nil {
			config.AdaptiveGaussian.ThresholdShift = shift
		}
	}
}
