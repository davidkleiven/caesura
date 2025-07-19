package pkg

import (
	"bytes"
	"io"
	"os"
	"reflect"
	"testing"
)

func TestDefaultConfigIsValid(t *testing.T) {
	config := NewDefaultConfig()
	if err := config.Validate(); err != nil {
		t.Errorf("default config should be valid, got error: %v", err)
	}
}

func TestOverWriteFromFile(t *testing.T) {
	config := NewDefaultConfig()
	content := `store_type: local-fs
local_fs:
  directory: /tmp/caesura`

	f, err := os.CreateTemp("", "config_test*.yaml")
	if err != nil {
		t.Error(err)
		return
	}
	defer os.Remove(f.Name())

	_, err = io.Copy(f, bytes.NewBufferString(content))
	f.Close()

	if err != nil {
		t.Error(err)
		return
	}

	// Load the config from the file
	loadedConfig, err := OverrideFromFile(f.Name(), config)
	if err != nil {
		t.Fatalf("failed to load config from file: %v", err)
	}

	// Validate the overwritten config
	if err := config.Validate(); err != nil {
		t.Errorf("overwritten config should be valid, got error: %v", err)
		return
	}

	if loadedConfig.StoreType != "local-fs" {
		t.Errorf("expected store_type to be 'local-fs', got '%s'", loadedConfig.StoreType)
	}
	if loadedConfig.LocalFS.Directory != "/tmp/caesura" {
		t.Errorf("expected local_fs.directory to be '/tmp/caesura', got '%s'", loadedConfig.LocalFS.Directory)
	}

	// Just confirm that load configuration works
	LoadConfig(f.Name())
}

func TestInvalidConfig(t *testing.T) {
	config := NewDefaultConfig()
	config.StoreType = "invalid-store-type"

	if err := config.Validate(); err == nil {
		t.Error("expected validation to fail for invalid store_type, but it succeeded")
	}

	config.StoreType = "local-fs"
	config.LocalFS.Directory = ""

	if err := config.Validate(); err == nil {
		t.Error("expected validation to fail for missing local_fs.directory, but it succeeded")
	}
}

func TestDefaultConfigAndErrorForNonExistingFile(t *testing.T) {
	config := NewDefaultConfig()
	_, err := OverrideFromFile("non_existing_file.yaml", config)
	if err == nil {
		t.Error("expected error when loading from non-existing file, but got none")
	}

	if err := config.Validate(); err != nil {
		t.Errorf("default config should be valid, got error: %v", err)
	}
}

func TestDefaultConfigWhenInvalidYamlContent(t *testing.T) {
	content := "invalid_yaml_content"
	f, err := os.CreateTemp("", "config_test*.yaml")
	if err != nil {
		t.Error(err)
		return
	}
	defer os.Remove(f.Name())
	_, err = io.Copy(f, bytes.NewBufferString(content))
	f.Close()
	if err != nil {
		t.Error(err)
		return
	}
	// Load the config from the file
	config, err := OverrideFromFile(f.Name(), NewDefaultConfig())
	if err == nil {
		t.Error("expected error when loading invalid YAML content, but got none")
		return
	}

	if err := config.Validate(); err != nil {
		t.Errorf("expected validation to fail for invalid YAML content, but it succeeded: %v", err)
		return
	}
}

func TestGetStore(t *testing.T) {
	config := NewDefaultConfig()
	config.StoreType = "in-memory"
	store := GetStore(config)
	if _, ok := store.(*InMemoryStore); !ok {
		t.Errorf("expected store to be of type InMemoryStore, got %T", store)
	}
}

func TestOverrideFromEnv(t *testing.T) {
	env := map[string]string{
		"CAESURA_TIMEOUT":      "1000",
		"CAESURA_SECRETS_PATH": "/secrets/",
	}

	getter := func(key string) (string, bool) {
		value, ok := env[key]
		return value, ok
	}

	config := NewDefaultConfig()
	OverrideFromEnv(config, getter)

	if config.SecretsPath != "/secrets/" {
		t.Fatalf("Expected secrets path to be '/secrets/' got '%s'", config.SecretsPath)
	}

	if config.Timeout != 1000 {
		t.Fatalf("Expected timeout to be '1000' got '%v'", config.Timeout)
	}
}

func TestLoadConfigReturnDefaultConfigOnError(t *testing.T) {
	config, err := LoadConfig("/some-random-config-file/")
	if err == nil {
		t.Fatalf("Expected error to occur")
	}

	defaultConfig := NewDefaultConfig()
	if !reflect.DeepEqual(config, defaultConfig) {
		t.Fatalf("Expected config to be equal to\n%+v\ngot\n%+v\n", defaultConfig, config)
	}
}
