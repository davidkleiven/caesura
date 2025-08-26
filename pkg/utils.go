package pkg

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"io"
	"math/rand"
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

type ZipAppender struct {
	readers []*zip.Reader
	err     error
}

func (za *ZipAppender) Add(zipBytes []byte) *ZipAppender {
	if za.err != nil {
		return za
	}

	var reader *zip.Reader
	reader, za.err = zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
	za.readers = append(za.readers, reader)
	return za
}

// Merge merges all the added zip files. In case of name collisions the file from
// the first added zip archive is retained
func (za *ZipAppender) Merge() ([]byte, error) {
	if za.err != nil {
		return []byte{}, za.err
	}
	var combined bytes.Buffer
	writer := zip.NewWriter(&combined)

	existingFiles := make(map[string]struct{})
	for _, reader := range za.readers {
		for _, file := range reader.File {
			if _, ok := existingFiles[file.Name]; ok {
				continue
			} else {
				existingFiles[file.Name] = struct{}{}
			}
			if err := copyZipEntry(file, writer); err != nil {
				za.err = err
			}
		}
	}
	writer.Close()
	return combined.Bytes(), za.err
}

func NewZipAppender() *ZipAppender {
	return &ZipAppender{
		readers: []*zip.Reader{},
	}
}

func copyZipEntry(file *zip.File, zw *zip.Writer) error {
	rc, err := file.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	header := file.FileHeader
	header.Method = zip.Deflate

	w, err := zw.CreateHeader(&header)
	if err != nil {
		return err
	}

	_, err = io.Copy(w, rc)
	return err
}

func RandomInsecureID(n int) string {
	letters := []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")
	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

func ReturnOnFirstError(fns ...func() error) error {
	for _, fn := range fns {
		if err := fn(); err != nil {
			return err
		}
	}
	return nil
}
