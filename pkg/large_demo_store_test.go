package pkg

import (
	"testing"

	"github.com/davidkleiven/caesura/testutils"
)

func TestLargeDemoStore(t *testing.T) {
	store := NewLargeDemoStore()
	testutils.AssertEqual(t, len(store.Organizations), 5)
}
