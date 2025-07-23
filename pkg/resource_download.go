package pkg

import (
	"archive/zip"
	"context"
	"io"
)

type ResourceDownloader struct {
	meta          *MetaData
	contentReader io.Reader
	zipFile       *zip.File
	err           error
}

func (r *ResourceDownloader) GetMetaData(ctx context.Context, store ResourceGetter, orgId, id string) *ResourceDownloader {
	if r.err != nil {
		return r
	}
	r.meta, r.err = store.MetaById(ctx, orgId, id)
	return r
}

func (r *ResourceDownloader) GetResource(ctx context.Context, store ResourceGetter, orgId string) *ResourceDownloader {
	if r.err != nil {
		return r
	}
	r.contentReader, r.err = store.Resource(ctx, orgId, r.meta.ResourceName())
	return r
}

func (r *ResourceDownloader) Content() (io.Reader, error) {
	return r.contentReader, r.err
}

func (r *ResourceDownloader) ExtractSingleFile(filename string) *ResourceDownloader {
	if r.err != nil {
		return r
	}
	r.zipFile, r.err = NewFileFromZipper().ReadBytes(r.contentReader).AsZip().GetFile(filename)
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

func (r *ResourceDownloader) ZipReader() (*zip.Reader, error) {
	if r.err != nil {
		return &zip.Reader{}, r.err
	}
	zipper := NewFileFromZipper().ReadBytes(r.contentReader).AsZip()
	return zipper.zipReader, zipper.err
}

func NewResourceDownloader() *ResourceDownloader {
	return &ResourceDownloader{
		meta: &MetaData{},
	}
}
