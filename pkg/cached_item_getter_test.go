package pkg

import (
	"context"
	"errors"
	"testing"

	"github.com/davidkleiven/caesura/testutils"
)

type EmptyItemGetter struct {
	Err error
}

func (e *EmptyItemGetter) Item(ctx context.Context, path string) ([]byte, error) {
	return []byte{}, e.Err
}

func TestCachedGetter(t *testing.T) {
	cachedGetter := NewCachedItemGetter(&EmptyItemGetter{})
	path := "/path/to/resource"
	_, err := cachedGetter.Item(context.Background(), path)
	testutils.AssertNil(t, err)
	testutils.AssertEqual(t, cachedGetter.Monitor.MaxSize, 1)
	testutils.AssertEqual(t, cachedGetter.Monitor.NumMisses, 1)
	testutils.AssertEqual(t, cachedGetter.Monitor.NumHits, 0)

	_, err = cachedGetter.Item(context.Background(), path)
	testutils.AssertNil(t, err)
	testutils.AssertEqual(t, cachedGetter.Monitor.MaxSize, 1)
	testutils.AssertEqual(t, cachedGetter.Monitor.NumMisses, 1)
	testutils.AssertEqual(t, cachedGetter.Monitor.NumHits, 1)

	cachedGetter.Clear("non-existing")
	testutils.AssertEqual(t, len(cachedGetter.cache), 1)

	cachedGetter.Clear(path)
	testutils.AssertEqual(t, len(cachedGetter.cache), 0)
}

func TestCachedGetterError(t *testing.T) {
	cachedGetter := NewCachedItemGetter(&EmptyItemGetter{Err: errors.New("something went wrong")})
	path := "/path/to/resource"
	_, err := cachedGetter.Item(context.Background(), path)
	if err == nil {
		t.Fatal("Expected error")
	}
	testutils.AssertEqual(t, cachedGetter.Monitor.MaxSize, 0)
	testutils.AssertEqual(t, cachedGetter.Monitor.NumMisses, 1)
	testutils.AssertEqual(t, cachedGetter.Monitor.NumHits, 0)
}
