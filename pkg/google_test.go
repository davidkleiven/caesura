package pkg

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"iter"
	"path"
	"strings"
	"sync"
	"testing"

	"cloud.google.com/go/firestore"
	"cloud.google.com/go/storage"
	"github.com/davidkleiven/caesura/testutils"
	"google.golang.org/api/iterator"
)

type LocalBucketClient struct {
	buckets map[string][]byte
	mutex   sync.Mutex
}

func NewLocalBucketClient() *LocalBucketClient {
	return &LocalBucketClient{
		buckets: map[string][]byte{},
	}
}

func (l *LocalBucketClient) Upload(ctx context.Context, bucket, object string, data []byte) error {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	fullName := bucket + "/" + object
	l.buckets[fullName] = data
	return nil
}

func (l *LocalBucketClient) GetObject(ctx context.Context, bucket, objName string) (io.ReadCloser, error) {
	location := path.Join(bucket, objName)
	data, ok := l.buckets[location]
	if !ok {
		return nil, fmt.Errorf("%s not found", location)
	}
	return io.NopCloser(bytes.NewBuffer(data)), nil
}

func (l *LocalBucketClient) GetObjects(ctx context.Context, bucket string, query *storage.Query) ObjectLister {
	prefix := path.Join(bucket, query.Prefix)

	items := []storage.ObjectAttrs{}
	for name := range l.buckets {
		if strings.HasPrefix(name, prefix) {
			items = append(items, storage.ObjectAttrs{Name: name})
		}
	}
	return &LocalObjectLister{items: items}
}

type LocalObjectLister struct {
	items []storage.ObjectAttrs
}

func (lo *LocalObjectLister) Next() (*storage.ObjectAttrs, error) {
	if len(lo.items) > 0 {
		item := lo.items[0]
		lo.items = lo.items[1:]
		return &item, nil
	}
	return nil, iterator.Done
}

type FailingBucketClient struct {
	uploadErr  error
	objReadErr error
}

func (f *FailingBucketClient) Upload(ctx context.Context, bucket, object string, data []byte) error {
	return f.uploadErr
}

func (f *FailingBucketClient) GetObject(ctx context.Context, bucket, objName string) (io.ReadCloser, error) {
	var buf bytes.Buffer
	return io.NopCloser(&buf), f.objReadErr
}

func (f *FailingBucketClient) GetObjects(ctx context.Context, bucket string, query *storage.Query) ObjectLister {
	return &LocalObjectLister{items: []storage.ObjectAttrs{}}
}

type SubmitTestData struct {
	store GoogleStore
	orgId string
	meta  *MetaData
	data  iter.Seq2[string, []byte]
}

func createSubmitData(bucketClient GoogleBucketClient, fsClient FirestoreClient) *SubmitTestData {
	config := NewTestConfig()
	store := GoogleStore{
		Config:       config,
		BucketClient: bucketClient,
		FsClient:     fsClient,
	}

	orgId := RandomInsecureID()
	meta := MetaData{
		Title:    "demo-score",
		Arranger: "John Doe",
		Composer: "Frankie Boy",
	}

	iter := func(yield func(n string, d []byte) bool) {
		for i := range 2 {
			name := fmt.Sprintf("data%d.pdf", i)
			content := []byte("some content")
			if !yield(name, content) {
				return
			}
		}
	}
	return &SubmitTestData{
		store: store,
		orgId: orgId,
		meta:  &meta,
		data:  iter,
	}
}

func TestGoogleSubmit(t *testing.T) {
	client := NewLocalBucketClient()
	fsClient := NewLocalFirestoreClient()
	submitData := createSubmitData(client, fsClient)

	err := submitData.store.Submit(context.Background(), submitData.orgId, submitData.meta, submitData.data)
	testutils.AssertNil(t, err)
	testutils.AssertEqual(t, len(client.buckets), 2)

	resourceId := submitData.meta.ResourceId()
	loc := path.Join(metaDataCollection, submitData.orgId, resourceId)
	data, ok := fsClient.data[loc]
	testutils.AssertEqual(t, ok, true)

	casted, ok := data.(*FirestoreMetaData)
	testutils.AssertEqual(t, ok, true)
	testutils.AssertEqual(t, casted.Status, StoreStatusFinished)
}

func TestGoogleSubmitBucketUploadError(t *testing.T) {
	client := FailingBucketClient{
		uploadErr: errors.New("something went wrong"),
	}
	fsClient := NewLocalFirestoreClient()
	submitData := createSubmitData(&client, fsClient)

	err := submitData.store.Submit(context.Background(), submitData.orgId, submitData.meta, submitData.data)
	if !errors.Is(err, client.uploadErr) {
		t.Fatalf("Wanted error to be %s got %s", client.uploadErr, err)
	}

	resourceId := submitData.meta.ResourceId()
	loc := path.Join(metaDataCollection, submitData.orgId, resourceId)
	data, ok := fsClient.data[loc]
	testutils.AssertEqual(t, ok, true)

	casted, ok := data.(*FirestoreMetaData)
	testutils.AssertEqual(t, ok, true)
	testutils.AssertEqual(t, casted.Status, StoreStatusPending)
}

type FailingFirestoreClient struct {
	errStoreDoc    error
	errUpdateField error
	errGetDoc      error
}

func (f *FailingFirestoreClient) StoreDocument(context context.Context, org, col, doc string, data any) error {
	return f.errStoreDoc
}

func (f *FailingFirestoreClient) Update(ctx context.Context, org, col, doc string, update []firestore.Update) error {
	return f.errUpdateField
}

func (f *FailingFirestoreClient) GetDocByPrefix(ctx context.Context, dataset, orgId, field, prefix string) iter.Seq[Document] {
	return func(yield func(doc Document) bool) {}
}

func (f *FailingFirestoreClient) GetDoc(ctx context.Context, dataset, orgId, itemid string) (Document, error) {
	return nil, f.errGetDoc
}

func TestNoBucketUploadOnMetaDataError(t *testing.T) {

	client := NewLocalBucketClient()
	fsClient := FailingFirestoreClient{errStoreDoc: errors.New("something went wrong")}
	submitData := createSubmitData(client, &fsClient)

	err := submitData.store.Submit(context.Background(), submitData.orgId, submitData.meta, submitData.data)
	if err == nil {
		t.Fatal("Expected error")
	}
	testutils.AssertEqual(t, len(client.buckets), 0)
}

func TestSubmitProjectGoogleStore(t *testing.T) {
	client := NewLocalFirestoreClient()
	project := Project{Name: "my-project"}
	store := GoogleStore{FsClient: client}
	store.SubmitProject(context.Background(), "my-org", &project)
	_, ok := client.data["projects/my-org/myproject"]
	testutils.AssertEqual(t, ok, true)
}

func TestGoogleMetaByPattern(t *testing.T) {
	client := NewLocalFirestoreClient()
	store := GoogleStore{FsClient: client}
	meta := MetaData{
		Title:    "With a smile and a song",
		Composer: "Frank Churchill",
		Arranger: "Unknown",
	}
	ctx := context.Background()
	pdfIter := func(yield func(n string, c []byte) bool) {}
	err := store.Submit(ctx, "my-org", &meta, pdfIter)
	testutils.AssertNil(t, err)

	for _, test := range []struct {
		pattern *MetaData
		wantNum int
		desc    string
	}{
		{
			pattern: &MetaData{Title: "with"},
			wantNum: 1,
			desc:    "Filter by title",
		},

		{
			pattern: &MetaData{Arranger: "un"},
			wantNum: 1,
			desc:    "Filter by arranger",
		},
		{
			pattern: &MetaData{Composer: "fra"},
			wantNum: 1,
			desc:    "Filter by composer",
		},
		{
			pattern: &MetaData{Title: "with", Composer: "fra"},
			wantNum: 1,
			desc:    "Filter by title and composer",
		},
		{
			pattern: &MetaData{},
			wantNum: 0,
			desc:    "No filter",
		},
	} {
		t.Run(test.desc, func(t *testing.T) {
			items, err := store.MetaByPattern(ctx, "my-org", test.pattern)
			testutils.AssertNil(t, err)
			testutils.AssertEqual(t, len(items), test.wantNum)
		})
	}

}

func TestProjectsByName(t *testing.T) {
	project := Project{Name: "My cool project"}

	client := NewLocalFirestoreClient()
	store := GoogleStore{FsClient: client}
	ctx := context.Background()
	err := store.SubmitProject(ctx, "my-org", &project)
	testutils.AssertNil(t, err)

	res, err := store.ProjectsByName(ctx, "my-org", "my")
	testutils.AssertNil(t, err)
	testutils.AssertEqual(t, len(res), 1)
	testutils.AssertEqual(t, res[0].Name, project.Name)
}

func TestRemoveResourceFromProject(t *testing.T) {
	project := Project{Name: "project", ResourceIds: []string{"id1", "id2"}}
	client := NewLocalFirestoreClient()
	store := GoogleStore{FsClient: client}
	ctx := context.Background()
	err := store.SubmitProject(ctx, "my-org", &project)
	testutils.AssertNil(t, err)
	err = store.RemoveResource(ctx, "my-org", "project", "id2")
	testutils.AssertNil(t, err)

	storedProject, err := store.ProjectById(ctx, "my-org", "project")
	testutils.AssertNil(t, err)
	testutils.AssertEqual(t, len(storedProject.ResourceIds), 1)
	testutils.AssertEqual(t, storedProject.ResourceIds[0], "id1")

}
