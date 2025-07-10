package pkg

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"io"
)

var ErrFileNotInZipArchive = errors.New("file is not in zip archive")

func PanicOnErr(err error) {
	if err != nil {
		panic(err)
	}
}

func RemoveDuplicates[T comparable](input []T) []T {
	seen := make(map[T]struct{})
	result := make([]T, 0, len(input))

	for _, v := range input {
		if _, ok := seen[v]; !ok {
			seen[v] = struct{}{}
			result = append(result, v)
		}
	}
	return result
}

func FileFromZip(r io.Reader, filename string) (*zip.File, error) {
	content, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	zipReader, err := zip.NewReader(bytes.NewReader(content), int64(len(content)))
	if err != nil {
		return nil, err
	}

	for _, file := range zipReader.File {
		if file.Name == filename {
			return file, nil
		}
	}
	return nil, errors.Join(ErrFileNotInZipArchive, fmt.Errorf(": file %s", filename))
}
