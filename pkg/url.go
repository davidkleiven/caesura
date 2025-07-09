package pkg

import (
	"errors"
	"fmt"
	"strings"
)

type InterpretedUrl struct {
	Path          string
	PathParameter string
}

var ErrCanNotInterpretUrl = errors.New("can not interpret url. Must have length 3")

func ParseUrl(url string) (InterpretedUrl, error) {
	tokens := strings.Split(url, "/")
	if len(tokens) != 3 {
		return InterpretedUrl{}, errors.Join(ErrCanNotInterpretUrl, fmt.Errorf("Got length %d", len(tokens)))
	}

	return InterpretedUrl{
		Path:          tokens[1],
		PathParameter: tokens[2],
	}, nil
}
