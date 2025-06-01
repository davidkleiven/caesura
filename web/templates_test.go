package web

import (
	"bytes"
	"testing"
)

func TestIndex(t *testing.T) {
	index := Index()

	if !bytes.Contains(index, []byte("Caesura</div>")) {
		t.Error("Expected index to contain 'Caesura</div>'")
	}
}
