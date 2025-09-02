package web

import (
	"testing"

	"github.com/davidkleiven/caesura/testutils"
)

func TestFieldValue(t *testing.T) {
	result := translator.MustGet("en", "non-existing-field")
	testutils.AssertEqual(t, result, "non-existing-field")
}

func TestAllFieldsExist(t *testing.T) {
	reference := translator.mapping["en"]
	for lang, langMapping := range translator.mapping {
		for f := range langMapping {
			_, exist := reference[f]
			if !exist {
				t.Fatalf("Field %s exist in %s, but not in en", f, lang)
			}
		}

		for f := range reference {
			_, exist := langMapping[f]
			if !exist {
				t.Fatalf("Field %s exist in en, but not in %s", f, lang)
			}
		}
	}
}

func TestPanicOnMissingFallback(t *testing.T) {
	tr := NewTranslator()
	delete(tr.mapping, "en")
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("Did not panic")
		}
	}()
	tr.MustGet("unknown-language", "field")
}
