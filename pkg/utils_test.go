package pkg

import (
	"errors"
	"testing"
)

func TestPanicOnErr(t *testing.T) {
	err := errors.New("test error")
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("PanicOnErr did not panic on error: %v", err)
		}
	}()

	PanicOnErr(err)
}
