package pkg

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"testing"
	"time"
)

func TestStore(t *testing.T) {
	fsStore, cleanup := mustCreateNewFsStore()
	defer cleanup()

	for _, test := range []struct {
		store Storer
		name  string
	}{
		{store: fsStore, name: "FSStore"},
	} {
		t.Run(test.name, func(t *testing.T) {
			data := []byte("test data")
			name := "testfile.txt"
			if err := test.store.Store(name, bytes.NewReader(data)); err != nil {
				t.Errorf("Store failed: %v", err)
				return
			}

			var contentReader io.Reader
			var err error
			switch store := test.store.(type) {
			case *FSStore:
				contentReader, err = store.Get(name)
				if err != nil {
					t.Errorf("Get failed: %v", err)
					return
				}
			default:
				t.Errorf("Unknown store type: %T", store)
				return
			}

			content, err := io.ReadAll(contentReader)
			if err != nil {
				t.Errorf("ReadAll failed: %v", err)
				return
			}
			if !bytes.Equal(content, data) {
				t.Errorf("Expected content %s, got %s", data, content)
				return
			}
		})
	}
}

type failingReader struct{}

func (f *failingReader) Read(p []byte) (n int, err error) {
	return 0, io.ErrUnexpectedEOF // Simulate a read error
}

func mustCreateNewFsStore() (*FSStore, func() error) {
	fsStore, err := os.MkdirTemp("", "fsstore")
	if err != nil {
		panic(err)
	}
	return NewFSStore(fsStore), func() error {
		return os.RemoveAll(fsStore)
	}
}

func TestStoreReaderFails(t *testing.T) {
	fsStore, cleanup := mustCreateNewFsStore()
	defer cleanup()
	for _, test := range []struct {
		store Storer
		name  string
	}{
		{store: fsStore, name: "FSStore"},
	} {
		t.Run(test.name, func(t *testing.T) {
			name := "testfile.txt"
			if err := test.store.Store(name, &failingReader{}); !errors.Is(err, ErrRetrievingContent) {
				t.Error("Expected ErrRetrievingContent, got:", err)
				return
			}
		})
	}
}

func TestFSStoreFailToCreate(t *testing.T) {
	store, cleanup := mustCreateNewFsStore()
	defer cleanup()
	name := "filename/with/path/testfile.txt"
	if err := store.Store(name, bytes.NewReader([]byte("test data"))); err == nil {
		t.Error("Expected error when storing file with path, got none")
		return
	}
}

func TestFSStoreGetNonExistingFile(t *testing.T) {
	store, cleanup := mustCreateNewFsStore()
	defer cleanup()

	name := "nonexistent.txt"
	reader, err := store.Get(name)
	if !errors.Is(err, ErrFileNotFound) {
		t.Error("Expected error when getting non-existing file, got none")
		return
	}

	content, err := io.ReadAll(reader)
	if err != nil {
		t.Errorf("Expected no content, got error: %v", err)
		return
	}
	if len(content) != 0 {
		t.Errorf("Expected empty content for non-existing file, got %d bytes", len(content))
		return
	}
}

func TestRegisterSuccessFS(t *testing.T) {
	store, cleanup := mustCreateNewFsStore()
	defer cleanup()
	meta := &MetaData{
		Status: StoreStatusPending,
		Title:  "test-resource",
	}

	if err := store.Register(meta); err != nil {
		t.Errorf("Register failed: %v", err)
		return
	}

	id := meta.ResourceId()
	loadedMeta := store.staged[id]

	if loadedMeta.Status != StoreStatusPending {
		t.Errorf("Expected status to be Pending, got %s", loadedMeta.Status)
		return
	}

	if err := store.RegisterSuccess(id); err != nil {
		t.Errorf("RegisterSuccess failed: %v", err)
		return
	}

	sidecar, err := os.Open(store.directory + "/testresource.json")
	if err != nil {
		t.Errorf("Failed to open sidecar file: %v", err)
		return
	}
	defer sidecar.Close()
	var updatedMeta MetaData
	if err := json.NewDecoder(sidecar).Decode(&updatedMeta); err != nil {
		t.Errorf("Failed to decode sidecar file: %v", err)
		return
	}

	if updatedMeta.Status != StoreStatusFinished {
		t.Errorf("Expected status to be Finished, got %s", updatedMeta.Status)
		return
	}
}

func TestErrorOnNoMetadata(t *testing.T) {
	fsStore, cleanup := mustCreateNewFsStore()
	defer cleanup()

	for _, test := range []struct {
		store Storer
		name  string
	}{
		{store: fsStore, name: "FSStore"},
	} {
		t.Run(test.name, func(t *testing.T) {
			id := "non-existing-id"
			if err := test.store.RegisterSuccess(id); !errors.Is(err, ErrResourceMetadataNotFound) {
				t.Errorf("Expected ErrResourceMetadataNotFound, got: %v", err)
				return
			}
		})
	}
}

func TestErrorOnDuplicateEntries(t *testing.T) {
	store, cleanup := mustCreateNewFsStore()
	defer cleanup()
	meta := &MetaData{
		Status: StoreStatusPending,
		Title:  "test-resource",
	}

	if err := store.Register(meta); err != nil {
		t.Errorf("First Register failed: %v", err)
		return
	}

	if err := store.Register(meta); !errors.Is(err, ErrUpdateMetadata) {
		t.Errorf("Expected ErrUpdateMetadata, got: %v", err)
		return
	}
}

func TestMetaDataString(t *testing.T) {
	for i, test := range []struct {
		metaData MetaData
		expected string
	}{
		{MetaData{Title: "Title", Composer: "Composer", Arranger: "Arranger"}, "title_composer_arranger.zip"},
		{MetaData{Title: "", Composer: "", Arranger: ""}, ".zip"},
		{MetaData{Title: "Title", Composer: "", Arranger: ""}, "title.zip"},
		{MetaData{Title: "", Composer: "Composer", Arranger: ""}, "composer.zip"},
		{MetaData{Title: "", Composer: "", Arranger: "Arranger"}, "arranger.zip"},
	} {
		m := test.metaData
		result := m.ResourceName()
		if result != test.expected {
			t.Errorf("Test %d failed. Expected '%s', got '%s'", i, test.expected, result)
		}
	}
}

func TestJsonMarshalingMetaData(t *testing.T) {
	meta := &MetaData{
		Title:    "Test Title",
		Composer: "Test Composer",
		Arranger: "Test Arranger",
	}

	data, err := json.Marshal(meta)
	if err != nil {
		t.Error(err)
	}

	// Expect an ID and a resource name in the JSON output
	if !bytes.Contains(data, []byte(meta.ResourceId())) {
		t.Errorf("Expected JSON to contain resource ID '%s', got %s", meta.ResourceId(), data)
	}
	if !bytes.Contains(data, []byte(meta.ResourceName())) {
		t.Errorf("Expected JSON to contain resource name '%s', got %s", meta.ResourceName(), data)
	}
}

func TestJsonUnmarshalingErrorOnInconsistency(t *testing.T) {
	meta := &MetaData{
		Title:    "Test Title",
		Composer: "Test Composer",
		Arranger: "Test Arranger",
	}

	data, err := json.Marshal(meta)
	if err != nil {
		t.Error(err)
	}

	for i, replace := range []string{meta.ResourceName(), meta.ResourceId()} {
		// Modify the resource name in the JSON data
		modifiedData := bytes.Replace(data, []byte(replace), []byte("some-modified-stuff"), 1)

		var newMeta MetaData
		if err := json.Unmarshal(modifiedData, &newMeta); err == nil {
			t.Errorf("Test #%d: Expected error on unmarshaling with inconsistent resource name, got none", i)
		}
	}
}

func TestMetaData_JSONRoundTrip(t *testing.T) {
	original := MetaData{
		Title:           "Blue Monk",
		Composer:        "Thelonious Monk",
		Arranger:        "John Doe",
		Genre:           "Jazz",
		Year:            "1959",
		Instrumentation: "Piano Trio",
		Duration:        2*time.Minute + 30*time.Second,
		Publisher:       "Jazz Press",
		Isnm:            "979-0-060-11561-5",
		Tags:            "bebop,standard",
		Notes:           "A jazz standard often played in jam sessions.",
		Status:          StoreStatusFinished,
	}

	// Marshal to JSON
	data, err := json.Marshal(&original)
	if err != nil {
		t.Fatalf("failed to marshal MetaData: %v", err)
	}

	// Unmarshal back
	var decoded MetaData
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal MetaData: %v", err)
	}

	// Compare the important fields
	if original != decoded {
		t.Errorf("round-trip mismatch:\nOriginal: %+v\nDecoded: %+v", original, decoded)
	}
}

func TestUnmarshalMetDataInvalidJSON(t *testing.T) {
	invalidJSON := []byte("Not JSON")

	var meta MetaData
	err := meta.UnmarshalJSON(invalidJSON)
	if err == nil {
		t.Error("Expected error on unmarshaling invalid JSON, got none")
		return
	}
}
