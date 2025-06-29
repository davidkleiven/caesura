package pkg

import (
	"bytes"
	"errors"
	"io"
	"os"
	"testing"
)

func TestStore(t *testing.T) {
	fsStore, err := os.MkdirTemp("", "fsstore")
	if err != nil {
		t.Error(err)
		return
	}
	defer os.RemoveAll(fsStore)

	for _, test := range []struct {
		store Storer
		name  string
	}{
		{store: NewInMemoryStore(), name: "InMemoryStore"},
		{store: NewFSStore(fsStore), name: "FSStore"},
	} {
		t.Run(test.name, func(t *testing.T) {
			data := []byte("test data")
			name := "testfile.txt"
			if err := test.store.Store(name, bytes.NewReader(data)); err != nil {
				t.Errorf("Store failed: %v", err)
				return
			}

			files, err := test.store.List("te")
			if err != nil || len(files) == 0 {
				t.Errorf("List failed: %v", err)
				return
			}
			if len(files) != 1 || files[0] != name {
				t.Errorf("Expected 1 file named %s, got %v", name, files)
				return
			}
			if r, err := test.store.Get(name); err != nil {
				t.Errorf("Get failed: %v", err)
				return
			} else {
				content, err := io.ReadAll(r)
				if err != nil {
					t.Errorf("ReadAll failed: %v", err)
					return
				}
				if !bytes.Equal(content, data) {
					t.Errorf("Expected content %s, got %s", data, content)
					return
				}
			}
		})
	}
}

func TestStoreDelete(t *testing.T) {
	fsStore, err := os.MkdirTemp("", "fsstore")
	if err != nil {
		t.Error(err)
		return
	}
	defer os.RemoveAll(fsStore)
	for _, test := range []struct {
		store Storer
		name  string
	}{
		{store: NewInMemoryStore(), name: "InMemoryStore"},
		{store: NewFSStore(fsStore), name: "FSStore"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := test.store.Delete("nonexistent.txt"); err != nil {
				t.Error(err)
				return
			}

			data := []byte("test data")
			name := "testfile.txt"
			if err := test.store.Store(name, bytes.NewReader(data)); err != nil {
				t.Errorf("Store failed: %v", err)
				return
			}

			if err := test.store.Delete(name); err != nil {
				t.Errorf("Delete failed: %v", err)
				return
			}

			if _, err := test.store.Get(name); err == nil {
				t.Errorf("Expected error when getting deleted file, got none")
				return
			}
		})
	}
}

type failingReader struct{}

func (f *failingReader) Read(p []byte) (n int, err error) {
	return 0, io.ErrUnexpectedEOF // Simulate a read error
}

func TestStoreReaderFails(t *testing.T) {
	fsStore, err := os.MkdirTemp("", "fsstore")
	if err != nil {
		t.Error(err)
		return
	}
	defer os.RemoveAll(fsStore)
	for _, test := range []struct {
		store Storer
		name  string
	}{
		{store: NewInMemoryStore(), name: "InMemoryStore"},
		{store: NewFSStore(fsStore), name: "FSStore"},
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
	fsStore, err := os.MkdirTemp("", "fsstore")
	if err != nil {
		t.Error(err)
		return
	}
	defer os.RemoveAll(fsStore)

	store := NewFSStore(fsStore)
	name := "filename/with/path/testfile.txt"
	if err := store.Store(name, bytes.NewReader([]byte("test data"))); err == nil {
		t.Error("Expected error when storing file with path, got none")
		return
	}
}

func TestFSStoreListNonExistingDir(t *testing.T) {

	store := NewFSStore("/nonexistent/directory")
	prefix := "nonexistent"
	files, err := store.List(prefix)
	if err == nil {
		t.Error("Expected error when listing files in non-existing directory, got none")
		return
	}

	if len(files) != 0 {
		t.Errorf("Expected no files, got %d files", len(files))
		return
	}

}

func TestFSStoreGetNonExistingFile(t *testing.T) {

	store := NewFSStore("/nonexistent/directory")
	name := "nonexistent.txt"
	reader, err := store.Get(name)
	if err == nil {
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
