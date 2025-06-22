package config

import (
	"strings"
	"testing"
)

func TestUpdateFromReader(t *testing.T) {
	config := NewDefaultConfig()

	yamlData := `port: ":9090"
max_request_size_mb: 20
adaptive_gaussian:
  block_size: 25
  threshold_shift: 3`

	if err := UpdateFromReader(config, strings.NewReader(yamlData)); err != nil {
		t.Fatalf("Failed to update config from reader: %v", err)
	}

	if config.Port != ":9090" {
		t.Errorf("Expected port ':9090', got '%s'", config.Port)
	}
	if config.MaxRequestSizeMB != 20 {
		t.Errorf("Expected max_request_size_mb 20, got %d", config.MaxRequestSizeMB)
	}
	if config.AdaptiveGaussian.BlockSize != 25 {
		t.Errorf("Expected adaptive_gaussian.block_size 25, got %d", config.AdaptiveGaussian.BlockSize)
	}
	if config.AdaptiveGaussian.ThresholdShift != 3 {
		t.Errorf("Expected adaptive_gaussian.threshold_shift 3, got %d", config.AdaptiveGaussian.ThresholdShift)
	}
}

func TestUpdateFromEnv(t *testing.T) {
	config := NewDefaultConfig()

	// Set environment variables
	t.Setenv("PORT", "8081")
	t.Setenv("MAX_REQUEST_SIZE", "15")
	t.Setenv("ADAPTIVE_GAUSSIAN_BLOCK_SIZE", "20")
	t.Setenv("ADAPTIVE_GAUSSIAN_THRESHOLD_SHIFT", "4")

	UpdateFromEnv(config)

	if config.Port != ":8081" {
		t.Errorf("Expected port ':8081', got '%s'", config.Port)
	}
	if config.MaxRequestSizeMB != 15 {
		t.Errorf("Expected max_request_size_mb 15, got %d", config.MaxRequestSizeMB)
	}
	if config.AdaptiveGaussian.BlockSize != 20 {
		t.Errorf("Expected adaptive_gaussian.block_size 20, got %d", config.AdaptiveGaussian.BlockSize)
	}
	if config.AdaptiveGaussian.ThresholdShift != 4 {
		t.Errorf("Expected adaptive_gaussian.threshold_shift 4, got %d", config.AdaptiveGaussian.ThresholdShift)
	}
}
