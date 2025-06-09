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

func TestList(t *testing.T) {
	list := List()

	if !bytes.Contains(list, []byte("</ul>")) {
		t.Error("Expected list to contain '</ul>'")
	}
}
