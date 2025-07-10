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

type FileFromZipper struct {
	content   []byte
	zipReader *zip.Reader
	err       error
}

func (f *FileFromZipper) ReadBytes(r io.Reader) *FileFromZipper {
	f.content, f.err = io.ReadAll(r)
	return f
}

func (f *FileFromZipper) AsZip() *FileFromZipper {
	if f.err != nil {
		return f
	}
	f.zipReader, f.err = zip.NewReader(bytes.NewReader(f.content), int64(len(f.content)))
	return f
}

func (f *FileFromZipper) GetFile(filename string) (*zip.File, error) {
	if f.err != nil {
		return &zip.File{}, f.err
	}

	for _, file := range f.zipReader.File {
		if file.Name == filename {
			return file, nil
		}
	}

	f.err = errors.Join(ErrFileNotInZipArchive, fmt.Errorf("file %s", filename))
	return &zip.File{}, f.err
}

func NewFileFromZipper() *FileFromZipper {
	return &FileFromZipper{}
}
