package pkg

import (
	"context"
	"errors"
	"fmt"
	"io"
	"iter"
	"path"
	"strings"
	"sync"

	"cloud.google.com/go/firestore"
	"cloud.google.com/go/storage"
)

const (
	metaDataCollection = "metadata"
	projectCollection  = "projects"
)

type GoogleConfig struct {
	Bucket    string `yaml:"bucket" env:"CAESURA_BUCKET"`
	ProjectId string `yaml:"projectId" env:"CAESURA_PROJECT_ID"`
}

func NewTestConfig() *GoogleConfig {
	return &GoogleConfig{
		Bucket:    "caesura-test",
		ProjectId: "caesura-466820",
	}

}

type ObjectLister interface {
	Next() (*storage.ObjectAttrs, error)
}

type GoogleBucketClient interface {
	Upload(ctx context.Context, bucket, object string, data []byte) error
	GetObject(ctx context.Context, bucket, objName string) (io.ReadCloser, error)
	GetObjects(ctx context.Context, bucket string, query *storage.Query) ObjectLister
}

type GCSBucketClient struct {
	client *storage.Client
}

func (g *GCSBucketClient) Upload(ctx context.Context, bucket, object string, data []byte) error {
	wc := g.client.Bucket(bucket).Object(object).NewWriter(ctx)
	if _, err := wc.Write(data); err != nil {
		return err
	}
	return wc.Close()
}

func (g *GCSBucketClient) GetObject(ctx context.Context, bucket, objName string) (io.ReadCloser, error) {
	return g.client.Bucket(bucket).Object(objName).NewReader(ctx)
}

func (g *GCSBucketClient) GetObjects(ctx context.Context, bucket string, query *storage.Query) ObjectLister {
	return g.client.Bucket(bucket).Objects(ctx, query)
}

type GoogleStore struct {
	BucketClient GoogleBucketClient
	FsClient     FirestoreClient
	Config       *GoogleConfig
}

func (gs *GoogleStore) objectName(orgId, resourceId, name string) string {
	return path.Join(orgId, resourceId, name)
}

func (gs *GoogleStore) Submit(ctx context.Context, orgId string, m *MetaData, pdfIter iter.Seq2[string, []byte]) error {
	var (
		wg       sync.WaitGroup
		firstErr error
		numErr   int
		mu       sync.Mutex
	)
	m.Status = StoreStatusPending

	metaRecord := FirestoreMetaData{
		MetaData:       *m,
		TitleSearch:    firebaseSearchString(m.Title),
		ComposerSearch: firebaseSearchString(m.Composer),
		ArrangerSearch: firebaseSearchString(m.Arranger),
	}

	resourceId := m.ResourceId()
	if err := gs.FsClient.StoreDocument(ctx, metaDataCollection, orgId, resourceId, &metaRecord); err != nil {
		return err
	}

	for name, data := range pdfIter {
		wg.Add(1)
		go func(file string, d []byte) {
			defer wg.Done()
			objName := gs.objectName(orgId, resourceId, file)
			err := gs.BucketClient.Upload(ctx, gs.Config.Bucket, objName, d)

			if err != nil {
				mu.Lock()
				firstErr = err
				numErr += 1
				mu.Unlock()
			}
		}(name, data)
	}
	wg.Wait()

	if firstErr != nil {
		return fmt.Errorf("Received %d errors. First error %w", numErr, firstErr)
	}
	return gs.FsClient.Update(
		ctx,
		metaDataCollection,
		orgId,
		resourceId,
		[]firestore.Update{{Path: "status", Value: StoreStatusFinished}},
	)
}

func (g *GoogleStore) SubmitProject(ctx context.Context, orgId string, project *Project) error {
	enrichedProject := FirestoreProject{
		Project:    *project,
		NameSearch: firebaseSearchString(project.Name),
	}
	return g.FsClient.StoreDocument(ctx, projectCollection, orgId, project.Id(), &enrichedProject)
}

func (g *GoogleStore) MetaByPattern(ctx context.Context, orgId string, pattern *MetaData) ([]MetaData, error) {
	result := []MetaData{}
	searchFields := []string{"title_search", "arranger_search", "composer_search"}
	prefixes := []string{pattern.Title, pattern.Arranger, pattern.Composer}
	seen := make(map[string]struct{})
	var err error

	for i := range len(searchFields) {
		if prefixes[i] == "" {
			continue
		}
		docIter := g.FsClient.GetDocByPrefix(ctx, metaDataCollection, orgId, searchFields[i], prefixes[i])
		for doc := range docIter {
			var meta MetaData
			currentErr := doc.DataTo(&meta)
			if currentErr != nil {
				err = errors.Join(err, currentErr)
				continue
			}

			resourceId := meta.ResourceId()
			if _, ok := seen[resourceId]; !ok {
				seen[resourceId] = struct{}{}
				result = append(result, meta)
			}
		}
	}
	return result, nil
}

func (g *GoogleStore) ProjectsByName(ctx context.Context, orgId string, name string) ([]Project, error) {
	docIter := g.FsClient.GetDocByPrefix(ctx, projectCollection, orgId, "name_search", name)
	projects := []Project{}
	var err error
	for doc := range docIter {
		var project Project
		currentErr := doc.DataTo(&project)
		if currentErr != nil {
			err = errors.Join(err, currentErr)
		} else {
			projects = append(projects, project)
		}
	}
	return projects, err
}

func (g *GoogleStore) ProjectById(ctx context.Context, orgId string, projectId string) (*Project, error) {
	doc, err := g.FsClient.GetDoc(ctx, projectCollection, orgId, projectId)
	if err != nil {
		return &Project{}, err
	}
	var proj Project
	err = doc.DataTo(&proj)
	return &proj, err
}

func (g *GoogleStore) RemoveResource(ctx context.Context, orgId string, projectId string, resourceId string) error {
	update := []firestore.Update{
		{
			Path:  "resource_ids",
			Value: firestore.ArrayRemove(resourceId),
		},
	}
	return g.FsClient.Update(ctx, projectCollection, orgId, projectId, update)
}

func firebaseSearchString(s string) string {
	s = strings.ToLower(s)
	s = strings.TrimPrefix(s, "the")
	s = strings.TrimSpace(s)
	return s
}

type FirestoreMetaData struct {
	MetaData
	TitleSearch    string `firestore:"title_search"`
	ComposerSearch string `firestore:"composer_search"`
	ArrangerSearch string `firestore:"arranger_search"`
}

type FirestoreProject struct {
	Project
	NameSearch string `firestore:"name_search"`
}
