package pkg

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/rand"
	"net/http"
	"time"

	"golang.org/x/text/language"
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

// 40 adjectives
var adjectives = []string{
	"brave", "calm", "eager", "fancy", "jolly", "kind", "lucky", "sunny", "witty", "clever",
	"bright", "shy", "happy", "bold", "gentle", "proud", "quick", "quiet", "smart", "zany",
	"curious", "serene", "cheerful", "mighty", "graceful", "playful", "silly", "loyal", "funny", "modest",
	"neat", "polite", "quirky", "tidy", "vivid", "warm", "zesty", "crafty", "jumpy", "sincere",
}

// 40 nouns
var nouns = []string{
	"lion", "tiger", "eagle", "panda", "unicorn", "otter", "dragon", "fox", "whale", "falcon",
	"wolf", "bear", "shark", "koala", "giraffe", "monkey", "sloth", "rabbit", "dolphin", "peacock",
	"camel", "leopard", "swan", "owl", "beetle", "penguin", "seal", "raccoon", "antelope", "kangaroo",
	"yak", "lizard", "rhino", "zebra", "turtle", "moose", "duck", "badger", "chimp", "crab",
}

var seededRand = rand.New(rand.NewSource(time.Now().UnixNano()))

func RandomInsecureID() string {
	adj := adjectives[seededRand.Intn(len(adjectives))]
	noun := nouns[seededRand.Intn(len(nouns))]
	now := time.Now().Unix()
	return fmt.Sprintf("%s-%s-%d", adj, noun, now)
}

func ReturnOnFirstError(fns ...func() error) error {
	for _, fn := range fns {
		if err := fn(); err != nil {
			return err
		}
	}
	return nil
}

func LanguageFromReq(r *http.Request) string {
	fallback := "en"
	accept := r.Header.Get("Accept-Language")
	if accept == "" {
		return fallback
	}

	supported := []language.Tag{
		language.English,
	}
	matcher := language.NewMatcher(supported)

	// Parse the Accept-Language header into a list of Tags
	tags, _, err := language.ParseAcceptLanguage(accept)
	if err != nil {
		slog.Error("Invalid accept header", "error", err)
		return fallback
	}

	// Match the best language
	bestMatch, _, _ := matcher.Match(tags...)
	return bestMatch.String()
}
