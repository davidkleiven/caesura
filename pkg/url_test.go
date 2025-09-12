package pkg

import (
	"errors"
	"testing"
)

func TestUrl(t *testing.T) {
	for _, test := range []struct {
		Url        string
		ShouldFail bool
		Expect     InterpretedUrl
	}{
		{
			Url:    "/resources/123",
			Expect: InterpretedUrl{Path: "resources", PathParameter: "123"},
		},
		{
			Url:        "/missing-resource",
			ShouldFail: true,
		},
	} {
		t.Run(test.Url, func(t *testing.T) {
			result, err := ParseUrl(test.Url)
			if test.ShouldFail && !errors.Is(err, ErrCanNotInterpretUrl) {
				t.Fatalf("Expected %s got %s", ErrCanNotInterpretUrl, err)
			}

			if !test.ShouldFail && result != test.Expect {
				t.Fatalf("Expected %+v got %+v\n", test.Expect, result)
			}
		})
	}
}
