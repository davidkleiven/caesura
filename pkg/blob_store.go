package pkg

import (
	"context"
	"iter"
	"time"
)

type MetaByPatternFetcher interface {
	MetaByPattern(ctx context.Context, orgId string, pattern *MetaData) ([]MetaData, error)
}

type ProjectByNameGetter interface {
	ProjectsByName(ctx context.Context, orgId string, name string) ([]Project, error)
}

type ProjectSubmitter interface {
	SubmitProject(ctx context.Context, orgId string, project *Project) error
}

type ProjectResourceRemover interface {
	RemoveResource(ctx context.Context, orgId string, projectId string, resourceId string) error
}

type MetaByIdGetter interface {
	MetaById(ctx context.Context, orgId string, id string) (*MetaData, error)
}

type ResourceGetter interface {
	MetaByIdGetter
	Resource(ctx context.Context, orgId string, path string) iter.Seq2[string, []byte]
}

type ItemGetter interface {
	Item(ctx context.Context, path string) ([]byte, error)
}

type BlobStore interface {
	Submitter
	MetaByPatternFetcher
	ProjectByNameGetter
	ProjectSubmitter
	ProjectMetaByIdGetter
	ProjectResourceRemover
	ResourceGetter
	ItemGetter
	SubscriptionStorer
	SubscriptionGetter
}

type SubscriptionValidator interface {
	SubscriptionGetter
	OrganizationGetter
}

type ProjectMetaByIdGetter interface {
	ProjectById(ctx context.Context, orgId string, id string) (*Project, error)
	MetaById(ctx context.Context, orgId string, id string) (*MetaData, error)
}

type Project struct {
	Name        string    `json:"name" firestore:"name"`
	ResourceIds []string  `json:"resource_ids" firestore:"resource_ids"`
	CreatedAt   time.Time `json:"created_at" firestore:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" firestore:"updated_at"`
}

func (p *Project) Merge(other *Project) {
	p.ResourceIds = RemoveDuplicates(append(p.ResourceIds, other.ResourceIds...))
	p.UpdatedAt = time.Now()
}

func (p *Project) Id() string {
	return SanitizeString(p.Name)
}
