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
				t.Fatalf("Store failed: %v", err)

			}

			var contentReader io.Reader
			var err error
			switch store := test.store.(type) {
			case *FSStore:
				contentReader, err = store.Get(name)
				if err != nil {
					t.Fatalf("Get failed: %v", err)

				}
			default:
				t.Fatalf("Unknown store type: %T", store)

			}

			content, err := io.ReadAll(contentReader)
			if err != nil {
				t.Fatalf("ReadAll failed: %v", err)

			}
			if !bytes.Equal(content, data) {
				t.Fatalf("Expected content %s, got %s", data, content)

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
				t.Fatal("Expected ErrRetrievingContent, got:", err)

			}
		})
	}
}

func TestFSStoreFailToCreate(t *testing.T) {
	store, cleanup := mustCreateNewFsStore()
	defer cleanup()
	name := "filename/with/path/testfile.txt"
	if err := store.Store(name, bytes.NewReader([]byte("test data"))); err == nil {
		t.Fatal("Expected error when storing file with path, got none")

	}
}

func TestFSStoreGetNonExistingFile(t *testing.T) {
	store, cleanup := mustCreateNewFsStore()
	defer cleanup()

	name := "nonexistent.txt"
	reader, err := store.Get(name)
	if !errors.Is(err, ErrFileNotFound) {
		t.Fatal("Expected error when getting non-existing file, got none")

	}

	content, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("Expected no content, got error: %v", err)

	}
	if len(content) != 0 {
		t.Fatalf("Expected empty content for non-existing file, got %d bytes", len(content))

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
		t.Fatalf("Register failed: %v", err)

	}

	id := meta.ResourceId()
	loadedMeta := store.staged[id]

	if loadedMeta.Status != StoreStatusPending {
		t.Fatalf("Expected status to be Pending, got %s", loadedMeta.Status)

	}

	if err := store.RegisterSuccess(id); err != nil {
		t.Fatalf("RegisterSuccess failed: %v", err)

	}

	sidecar, err := os.Open(store.directory + "/testresource.json")
	if err != nil {
		t.Fatalf("Failed to open sidecar file: %v", err)

	}
	defer sidecar.Close()
	var updatedMeta MetaData
	if err := json.NewDecoder(sidecar).Decode(&updatedMeta); err != nil {
		t.Fatalf("Failed to decode sidecar file: %v", err)

	}

	if updatedMeta.Status != StoreStatusFinished {
		t.Fatalf("Expected status to be Finished, got %s", updatedMeta.Status)

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
				t.Fatalf("Expected ErrResourceMetadataNotFound, got: %v", err)

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
		t.Fatalf("First Register failed: %v", err)

	}

	if err := store.Register(meta); !errors.Is(err, ErrUpdateMetadata) {
		t.Fatalf("Expected ErrUpdateMetadata, got: %v", err)

	}
}

func TestMetaDataString(t *testing.T) {
	for i, test := range []struct {
		metaData MetaData
		expected string
	}{
		{MetaData{Title: "Title", Composer: "Composer", Arranger: "Arranger"}, "title_composer_arranger"},
		{MetaData{Title: "", Composer: "", Arranger: ""}, ""},
		{MetaData{Title: "Title", Composer: "", Arranger: ""}, "title"},
		{MetaData{Title: "", Composer: "Composer", Arranger: ""}, "composer"},
		{MetaData{Title: "", Composer: "", Arranger: "Arranger"}, "arranger"},
	} {
		m := test.metaData
		result := m.ResourceId()
		if result != test.expected {
			t.Fatalf("Test %d failed. Expected '%s', got '%s'", i, test.expected, result)
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
		t.Fatal(err)
	}

	// Expect an ID and a resource name in the JSON output
	if !bytes.Contains(data, []byte(meta.ResourceId())) {
		t.Fatalf("Expected JSON to contain resource ID '%s', got %s", meta.ResourceId(), data)
	}
	if !bytes.Contains(data, []byte(meta.ResourceId())) {
		t.Fatalf("Expected JSON to contain resource name '%s', got %s", meta.ResourceId(), data)
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
		t.Fatal(err)
	}

	for i, replace := range []string{meta.ResourceId(), meta.ResourceId()} {
		// Modify the resource name in the JSON data
		modifiedData := bytes.Replace(data, []byte(replace), []byte("some-modified-stuff"), 1)

		var newMeta MetaData
		if err := json.Unmarshal(modifiedData, &newMeta); err == nil {
			t.Fatalf("Test #%d: Expected error on unmarshaling with inconsistent resource name, got none", i)
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
		Duration:        Duration(2*time.Minute + 30*time.Second),
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
		t.Fatalf("round-trip mismatch:\nOriginal: %+v\nDecoded: %+v", original, decoded)
	}
}

func TestUnmarshalMetDataInvalidJSON(t *testing.T) {
	invalidJSON := []byte("Not JSON")

	var meta MetaData
	err := meta.UnmarshalJSON(invalidJSON)
	if err == nil {
		t.Fatal("Expected error on unmarshaling invalid JSON, got none")

	}
}

func TestUnmarshalDurationString(t *testing.T) {
	jsonStr := []byte(`{"duration": "1m20s"}`)
	var meta MetaData
	if err := json.Unmarshal(jsonStr, &meta); err != nil {
		t.Fatal(err)
	}

	want := time.Minute + 20*time.Second
	if meta.Duration != Duration(want) {
		t.Fatalf("Duration should be set to %d got %d", meta.Duration, want)
	}
}

func TestUnmarshalCustomDuration(t *testing.T) {
	for _, test := range []struct {
		jsonBytes []byte
		want      Duration
		desc      string
	}{
		{
			jsonBytes: []byte(""),
			want:      Duration(0),
			desc:      "Empty string",
		},
		{
			jsonBytes: []byte("1m30s"),
			want:      Duration(time.Minute + 30*time.Second),
			desc:      "Empty string",
		},
	} {
		t.Run(test.desc, func(t *testing.T) {
			var d Duration
			err := d.UnmarshalJSON(test.jsonBytes)
			if err != nil {
				t.Fatal(err)
			}

			if d != test.want {
				t.Fatalf("Wanted %d got %d", test.want, d)
			}
		})
	}
}

func TestUnmarshalCustomDurationInvalidJson(t *testing.T) {
	jsonBytes := []byte("not json")
	var d Duration
	err := d.UnmarshalJSON(jsonBytes)
	if err == nil {
		t.Fatalf("Wanted error got %s", err)
	}
}

func TestCustomDurationString(t *testing.T) {
	if s := Duration(0).String(); s != "0s" {
		t.Fatalf("Wanted '0s' got '%s'", s)
	}
}

func TestMarshalUnMarshalRoundTrip(t *testing.T) {
	var d Duration
	b, err := d.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}

	var d2 Duration
	err = d2.UnmarshalJSON(b)
	if err != nil {
		t.Fatal(err)
	}

	if d != d2 {
		t.Fatalf("Wanted %d got %d", d, d2)
	}
}
