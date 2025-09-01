package pkg

import (
	"archive/zip"
	"context"
	"io"
	"iter"
	"strings"
)

type ZipWriter interface {
	Create(name string) (io.Writer, error)
	Close() error
}

type ResourceDownloader struct {
	meta        *MetaData
	contentIter iter.Seq2[string, []byte]
	zwFactory   func(w io.Writer) ZipWriter
	Error       error
}

func (r *ResourceDownloader) GetMetaData(ctx context.Context, store ResourceGetter, orgId, id string) *ResourceDownloader {
	if r.Error != nil {
		return r
	}
	r.meta, r.Error = store.MetaById(ctx, orgId, id)
	return r
}

func (r *ResourceDownloader) GetResource(ctx context.Context, store ResourceGetter, orgId string) *ResourceDownloader {
	if r.Error != nil {
		return r
	}
	r.contentIter = store.Resource(ctx, orgId, r.meta.ResourceId())
	return r
}

func (r *ResourceDownloader) ExtractSingleFile(filename string, w io.Writer) *ResourceDownloader {
	for name, file := range r.contentIter {
		if name == filename {
			if _, err := w.Write(file); err != nil {
				r.Error = err
				return r
			}
		}
	}
	return r
}

func (r *ResourceDownloader) ZipResource(w io.Writer) *ResourceDownloader {
	if r.Error != nil {
		return r
	}
	zw := r.zwFactory(w)
	defer zw.Close()
	resourceExist := false

	for name, content := range r.contentIter {
		resourceExist = true
		subwriter, err := zw.Create(name)
		if err != nil {
			r.Error = err
			return r
		}
		if _, err := subwriter.Write(content); err != nil {
			r.Error = err
			return r
		}
	}

	if !resourceExist {
		r.Error = ErrResourceNotFound
	}
	return r
}

func (r *ResourceDownloader) Filenames() []string {
	result := []string{}
	for name := range r.contentIter {
		splitted := strings.Split(name, "/")
		subname := splitted[len(splitted)-1]
		result = append(result, subname)
	}
	return result
}

func (r *ResourceDownloader) ZipFilename() string {
	return r.meta.ResourceId() + ".zip"
}

func NewResourceDownloader() *ResourceDownloader {
	return &ResourceDownloader{
		meta:        &MetaData{},
		contentIter: func(yield func(string, []byte) bool) {},
		zwFactory: func(w io.Writer) ZipWriter {
			return zip.NewWriter(w)
		},
	}
}
