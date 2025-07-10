package pkg

import (
	"archive/zip"
	"context"
	"errors"
	"io"
	"net/url"
)

var ErrResourceIdIsRequired = errors.New("resource id is required")

type ResourceDownloader struct {
	ResourceId    string
	File          string
	meta          *MetaData
	contentReader io.Reader
	zipFile       *zip.File
	err           error
}

func (r *ResourceDownloader) ParseUrl(url *url.URL) *ResourceDownloader {
	interpretedPath, err := ParseUrl(url.Path)
	r.File = url.Query().Get("file")
	if err != nil {
		r.err = ErrResourceIdIsRequired
		return r
	}

	r.ResourceId = interpretedPath.PathParameter
	return r
}

func (r *ResourceDownloader) GetMetaData(ctx context.Context, store ResourceGetter) *ResourceDownloader {
	if r.err != nil {
		return r
	}
	r.meta, r.err = store.MetaById(ctx, r.ResourceId)
	return r
}

func (r *ResourceDownloader) GetResource(ctx context.Context, store ResourceGetter) *ResourceDownloader {
	if r.err != nil {
		return r
	}
	r.contentReader, r.err = store.Resource(ctx, r.meta.ResourceName())
	return r
}

func (r *ResourceDownloader) Content() (io.Reader, error) {
	return r.contentReader, r.err
}

func (r *ResourceDownloader) ExtractSingleFile() *ResourceDownloader {
	if r.err != nil {
		return r
	}
	r.zipFile, r.err = NewFileFromZipper().ReadBytes(r.contentReader).AsZip().GetFile(r.File)
	return r
}

func (r *ResourceDownloader) FileReader() (io.ReadCloser, error) {
	if r.err != nil {
		return nil, r.err
	}

	return r.zipFile.Open()
}

func (r *ResourceDownloader) ZipFilename() string {
	return r.meta.ResourceName()
}

func (r *ResourceDownloader) SingleFileRequested() bool {
	return r.File != ""
}

func (r *ResourceDownloader) SingleFileName() string {
	return r.File
}

func NewResourceDownloader() *ResourceDownloader {
	return &ResourceDownloader{}
}
