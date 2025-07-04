package pkg

import "context"

type MetaByPatternFetcher interface {
	MetaByPattern(ctx context.Context, pattern *MetaData) ([]MetaData, error)
}

type BlobStore interface {
	Submitter
	MetaByPatternFetcher
}
