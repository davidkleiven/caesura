package pkg

import (
	"context"
	"io"
	"time"
)

type MetaByPatternFetcher interface {
	MetaByPattern(ctx context.Context, pattern *MetaData) ([]MetaData, error)
}

type ProjectByNameGetter interface {
	ProjectsByName(ctx context.Context, name string) ([]Project, error)
}

type ProjectSubmitter interface {
	SubmitProject(ctx context.Context, project *Project) error
}

type MetaByIdGetter interface {
	MetaById(ctx context.Context, id string) (*MetaData, error)
}

type ResourceGetter interface {
	MetaByIdGetter
	Resource(ctx context.Context, path string) (io.Reader, error)
}

type BlobStore interface {
	Submitter
	MetaByPatternFetcher
	ProjectByNameGetter
	ProjectSubmitter
	ProjectMetaByIdGetter
	ResourceGetter
}

type ProjectMetaByIdGetter interface {
	ProjectById(ctx context.Context, id string) (*Project, error)
	MetaById(ctx context.Context, id string) (*MetaData, error)
}

type Project struct {
	Name        string    `json:"name"`
	ResourceIds []string  `json:"resource_ids"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (p *Project) Merge(other *Project) {
	p.ResourceIds = RemoveDuplicates(append(p.ResourceIds, other.ResourceIds...))
	p.UpdatedAt = time.Now()
}

func (p *Project) Id() string {
	return SanitizeString(p.Name)
}
