package pkg

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/davidkleiven/caesura/testutils"
)

func TestDefaultConfigIsValid(t *testing.T) {
	config := NewDefaultConfig()
	if err := config.Validate(); err != nil {
		t.Fatalf("default config should be valid, got error: %v", err)
	}
}

func TestOverWriteFromFile(t *testing.T) {
	config := NewDefaultConfig()
	content := `store_type: local-fs
local_fs:
  directory: /tmp/caesura`

	f, err := os.CreateTemp("", "config_test*.yaml")
	if err != nil {
		t.Fatal(err)

	}
	defer os.Remove(f.Name())

	_, err = io.Copy(f, bytes.NewBufferString(content))
	f.Close()

	if err != nil {
		t.Fatal(err)

	}

	// Load the config from the file
	loadedConfig, err := OverrideFromFile(f.Name(), config)
	if err != nil {
		t.Fatalf("failed to load config from file: %v", err)
	}

	// Validate the overwritten config
	if err := config.Validate(); err != nil {
		t.Fatalf("overwritten config should be valid, got error: %v", err)

	}

	if loadedConfig.StoreType != "local-fs" {
		t.Fatalf("expected store_type to be 'local-fs', got '%s'", loadedConfig.StoreType)
	}
	if loadedConfig.LocalFS.Directory != "/tmp/caesura" {
		t.Fatalf("expected local_fs.directory to be '/tmp/caesura', got '%s'", loadedConfig.LocalFS.Directory)
	}

	// Just confirm that load configuration works
	LoadConfig(f.Name())
}

func TestInvalidConfig(t *testing.T) {
	config := NewDefaultConfig()
	config.StoreType = "invalid-store-type"

	if err := config.Validate(); err == nil {
		t.Fatal("expected validation to fail for invalid store_type, but it succeeded")
	}

	config.StoreType = "local-fs"
	config.LocalFS.Directory = ""

	if err := config.Validate(); err == nil {
		t.Fatal("expected validation to fail for missing local_fs.directory, but it succeeded")
	}
}

func TestDefaultConfigAndErrorForNonExistingFile(t *testing.T) {
	config := NewDefaultConfig()
	_, err := OverrideFromFile("non_existing_file.yaml", config)
	if err == nil {
		t.Fatal("expected error when loading from non-existing file, but got none")
	}

	if err := config.Validate(); err != nil {
		t.Fatalf("default config should be valid, got error: %v", err)
	}
}

func TestDefaultConfigWhenInvalidYamlContent(t *testing.T) {
	content := "invalid_yaml_content"
	f, err := os.CreateTemp("", "config_test*.yaml")
	if err != nil {
		t.Fatal(err)

	}
	defer os.Remove(f.Name())
	_, err = io.Copy(f, bytes.NewBufferString(content))
	f.Close()
	if err != nil {
		t.Fatal(err)

	}
	// Load the config from the file
	config, err := OverrideFromFile(f.Name(), NewDefaultConfig())
	if err == nil {
		t.Fatal("expected error when loading invalid YAML content, but got none")

	}

	if err := config.Validate(); err != nil {
		t.Fatalf("expected validation to fail for invalid YAML content, but it succeeded: %v", err)

	}
}

func TestGetStore(t *testing.T) {
	config := NewDefaultConfig()

	for _, storeType := range []string{"in-memory", "small-demo"} {
		config.StoreType = storeType
		store := GetStore(config)
		if _, ok := store.(*MultiOrgInMemoryStore); !ok {
			t.Fatalf("expected store to be of type InMemoryStore, got %T", store)
		}
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
	config.SmtpConfig.SendFn = nil
	if err == nil {
		t.Fatalf("Expected error to occur")
	}

	defaultConfig := NewDefaultConfig()
	defaultConfig.SmtpConfig.SendFn = nil
	if !reflect.DeepEqual(config, defaultConfig) {
		t.Fatalf("Expected config to be equal to\n%+v\ngot\n%+v\n", defaultConfig, config)
	}
}

func TestFileEnvGetter(t *testing.T) {
	tmp, err := os.CreateTemp("", "CAESURA_TIMEOUT")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmp.Name())
	tmp.Write([]byte("5000"))
	tmp.Close()

	dir := filepath.Dir(tmp.Name())
	getter := FileEnvGetter(dir)
	value, ok := getter(filepath.Base(tmp.Name()))
	if !ok {
		t.Fatal("Value was not OK")
	}
	if value != "5000" {
		t.Fatalf("Expected '5000' got '%s'", value)
	}
}

func TestOAuthGetter(t *testing.T) {
	config := NewDefaultConfig()
	oauth := config.OAuthConfig()
	if config.GoogleAuthClientId != oauth.ClientID {
		t.Fatalf("Wanted %s got %s", config.GoogleAuthClientId, oauth.ClientID)
	}
}

func TestGetGoogleStoreFromConfig(t *testing.T) {
	config := NewDefaultConfig()
	config.StoreType = GoogleCloud
	store := GetStore(config)
	_, ok := store.(*GoogleStore)
	testutils.AssertEqual(t, ok, true)
}

func TestOverrideEmailDeliveryService(t *testing.T) {
	config := NewDefaultConfig()
	_, err := OverrideEmailDeliveryService(config)
	testutils.AssertNil(t, err)

	config.EmailDeliveryService = "brevo"
	_, err = OverrideEmailDeliveryService(config)
	if err == nil {
		t.Fatal("Wanted error")
	}
	testutils.AssertContains(t, err.Error(), "brevo")

	config.BrevoApiKey = "some-api-key"
	_, err = OverrideEmailDeliveryService(config)
	testutils.AssertNil(t, err)
	testutils.AssertEqual(t, config.EmailSender, "noreply@caesura.no")
	testutils.AssertEqual(t, config.SmtpConfig.Host, "smtp-relay.brevo.com")
}

func TestLoadProfileNonExistent(t *testing.T) {
	_, err := LoadProfile("no-profile")
	if err == nil {
		t.Fatal("Wanted error")
	}
	testutils.AssertContains(t, err.Error(), "file does not")
}

func TestLoadProfile(t *testing.T) {
	// config-unittest require a special key see TestSuccessFullDecryption
	res, err := LoadProfile("config-unittest.yml")
	if err == nil {
		t.Fatal("Wanted error because no decryption key exists")
	}
	testutils.AssertContains(t, err.Error(), "decrypt")
	testutils.AssertEqual(t, res.BrevoApiKey, "")
}

func TestSuccessFullDecryption(t *testing.T) {
	key := "SOPS_AGE_KEY"
	orig, ok := os.LookupEnv(key)
	defer func() {
		if ok {
			os.Setenv(key, orig)
		} else {
			os.Unsetenv(key)
		}
	}()

	// Dummy key used to encrypt a test config file (not used for real files)
	testKey := "AGE-SECRET-KEY-1RXUZPSPY9VRV52XE867Q92DVL4C9YQWC2CXSSG224A5HJHRAKD6S2T3XH2"
	os.Setenv(key, testKey)
	res, err := LoadProfile("config-unittest.yml")
	testutils.AssertNil(t, err)
	testutils.AssertEqual(t, res.BrevoApiKey, "very-secret-key")
}

func TestGetMaxNumScoresUnknownPriceId(t *testing.T) {
	priceIds := NewTestPriceIds()
	testutils.AssertEqual(t, priceIds.NumScores("unknown-price-id"), 10)
}

func TestGetMaxNumScores(t *testing.T) {
	priceIds := NewProdPriceIds()
	testutils.AssertEqual(t, priceIds.NumScores(priceIds.Free), 10)
}
