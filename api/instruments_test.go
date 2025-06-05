package api

import (
	"slices"
	"testing"
)

func TestInstruments(t *testing.T) {
	result := Instruments("tru")
	want := []string{"Trumpet", "Piccolo Trumpet"}
	if slices.Compare(result, want) != 0 {
		t.Errorf("Wanted %v\ngot%v\n", want, result)
	}
}
