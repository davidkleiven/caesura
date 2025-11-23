package pkg

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"iter"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"cloud.google.com/go/firestore"
	"cloud.google.com/go/storage"
	"github.com/davidkleiven/caesura/testutils"
	"google.golang.org/api/iterator"
)

type LocalBucketClient struct {
	buckets map[string]io.ReadCloser
	mutex   sync.Mutex
}

func NewLocalBucketClient() *LocalBucketClient {
	return &LocalBucketClient{
		buckets: make(map[string]io.ReadCloser),
	}
}

func (l *LocalBucketClient) Upload(ctx context.Context, bucket, object string, data []byte) error {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	fullName := bucket + "/" + object
	l.buckets[fullName] = io.NopCloser(bytes.NewBuffer(data))
	return nil
}

func (l *LocalBucketClient) GetObject(ctx context.Context, bucket, objName string) (io.ReadCloser, error) {
	location := path.Join(bucket, objName)
	data, ok := l.buckets[location]
	if !ok {
		return nil, fmt.Errorf("%s not found", location)
	}
	return data, nil
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
	errDeleteDoc   error
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

func (f *FailingFirestoreClient) DeleteDoc(ctx context.Context, dataset, collection, itemId string) error {
	return f.errDeleteDoc
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

func storeWithMetaData() (*GoogleStore, error) {
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
	return &store, err
}

func TestGoogleMetaByPattern(t *testing.T) {
	store, err := storeWithMetaData()
	testutils.AssertNil(t, err)

	ctx := context.Background()
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

	if !storedProject.CreatedAt.Before(storedProject.UpdatedAt) {
		t.Fatalf("UpdatedAt (%s) is not after CreatedAt (%s)", storedProject.UpdatedAt, storedProject.CreatedAt)
	}
}

func TestGoogleMetaById(t *testing.T) {
	store, err := storeWithMetaData()
	testutils.AssertNil(t, err)

	t.Run("exsisting", func(t *testing.T) {
		metaId := "withasmileandasong_frankchurchill_unknown"
		meta, err := store.MetaById(context.Background(), "my-org", metaId)
		testutils.AssertNil(t, err)
		testutils.AssertContains(t, meta.Title, "With a ")
	})

	t.Run("non-exsisting", func(t *testing.T) {
		metaId := "non-existing"
		meta, err := store.MetaById(context.Background(), "my-org", metaId)
		testutils.AssertEqual(t, meta.Title, "")
		if err == nil {
			t.Fatal("Expected error")
		}
	})
}

func TestResource(t *testing.T) {
	validContent1 := bytes.NewBufferString("content1")
	validContent2 := bytes.NewBufferString("content3")
	failure := failingReader{}

	client := NewLocalBucketClient()
	client.buckets["test/org/resource1/content1.txt"] = io.NopCloser(validContent1)
	client.buckets["test/org/resource1/content2.txt"] = io.NopCloser(&failure)
	client.buckets["test/org/resource1/content3.txt"] = io.NopCloser(validContent2)

	store := GoogleStore{BucketClient: client, Config: &GoogleConfig{Bucket: "test"}}

	names := make(map[string]struct{})
	for name, item := range store.Resource(context.Background(), "org", "resource1") {
		names[name] = struct{}{}
		if len(item) == 0 {
			t.Fatal("Expected item to be zero")
		}
	}

	testutils.AssertEqual(t, len(names), 2)
	wantNames := []string{"content1.txt", "content3.txt"}
	for _, n := range wantNames {
		_, ok := names[n]
		if !ok {
			t.Fatalf("%s not in %v\n", n, names)
		}
	}
}

func TestGoogleItem(t *testing.T) {
	client := NewLocalBucketClient()

	content := bytes.NewBufferString("some text")
	client.buckets["test/obj1"] = io.NopCloser(content)

	store := GoogleStore{BucketClient: client, Config: &GoogleConfig{Bucket: "test"}}

	t.Run("success", func(t *testing.T) {
		readContent, err := store.Item(context.Background(), "obj1")
		testutils.AssertNil(t, err)
		testutils.AssertEqual(t, string(readContent), "some text")
	})

	t.Run("failure", func(t *testing.T) {
		readContent, err := store.Item(context.Background(), "obj2")
		if err == nil {
			t.Fatal("Wanted error")
		}
		testutils.AssertEqual(t, len(readContent), 0)
	})
}

func TestGoogleSubscriptions(t *testing.T) {
	store := GoogleStore{FsClient: NewLocalFirestoreClient()}
	sub := Subscription{Id: "my-sub"}
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		err := store.StoreSubscription(ctx, "org", &sub)
		testutils.AssertNil(t, err)

		res, err := store.GetSubscription(ctx, "org")
		testutils.AssertNil(t, err)
		testutils.AssertEqual(t, res.Id, sub.Id)
	})

	t.Run("failure", func(t *testing.T) {
		err := store.StoreSubscription(ctx, "org", &sub)
		testutils.AssertNil(t, err)

		res, err := store.GetSubscription(ctx, "non-existent")
		if err == nil {
			t.Fatal("Wanted error")
		}
		testutils.AssertEqual(t, res.Id, "")
	})
}

func TestRegisterOrganization(t *testing.T) {
	localClient := NewLocalFirestoreClient()
	store := GoogleStore{FsClient: localClient}
	org := Organization{Id: "my-org"}
	ctx := context.Background()
	err := store.RegisterOrganization(ctx, &org)
	testutils.AssertNil(t, err)

	t.Run("success", func(t *testing.T) {
		receivedOrg, err := store.GetOrganization(ctx, "my-org")
		testutils.AssertNil(t, err)
		testutils.AssertEqual(t, receivedOrg.Id, "my-org")
	})

	t.Run("failure", func(t *testing.T) {
		receivedOrg, err := store.GetOrganization(ctx, "non-existent-org")
		if err == nil {
			t.Fatal("Wanted error")
		}
		testutils.AssertEqual(t, receivedOrg.Id, "")
	})

	t.Run("delete", func(t *testing.T) {
		err := store.DeleteOrganization(ctx, "my-org")
		testutils.AssertNil(t, err)
		deletedOrg, ok := localClient.data[filepath.Join(organizationCollection, organizationInfo, "my-org")]
		testutils.AssertEqual(t, ok, true)
		org, ok := deletedOrg.(*Organization)
		testutils.AssertEqual(t, ok, true)
		testutils.AssertEqual(t, org.Deleted, true)
	})

}

func TestGoogleRegisterUser(t *testing.T) {
	fsClient := NewLocalFirestoreClient()
	store := GoogleStore{FsClient: fsClient}
	userInfo := UserInfo{
		Id: "test-user",
		Roles: map[string]RoleKind{
			"org1": RoleAdmin,
			"org2": RoleEditor,
		},
		Groups: map[string][]string{
			"org1": {"Saxophone", "Trombone"},
			"org2": {"Trumpet"},
		},
	}

	ctx := context.Background()
	err := store.RegisterUser(ctx, &userInfo)
	testutils.AssertNil(t, err)

	// Put wrong object into the store such that DataTo fails
	err = fsClient.StoreDocument(ctx, userCollection, userInfoDoc, "wrong-type", nil)
	testutils.AssertNil(t, err)

	t.Run("success", func(t *testing.T) {
		receivedUser, err := store.GetUserInfo(ctx, "test-user")
		testutils.AssertNil(t, err)
		testutils.AssertEqual(t, receivedUser.Id, "test-user")
		testutils.AssertEqual(t, receivedUser.Roles["org1"], RoleAdmin)
		testutils.AssertEqual(t, receivedUser.Roles["org2"], RoleEditor)
		testutils.AssertEqual(t, slices.Compare(receivedUser.Groups["org1"], []string{"Saxophone", "Trombone"}), 0)
		testutils.AssertEqual(t, slices.Compare(receivedUser.Groups["org2"], []string{"Trumpet"}), 0)

	})

	t.Run("user-not-found", func(t *testing.T) {
		receivedUser, err := store.GetUserInfo(ctx, "non-existing-user")
		if err == nil {
			t.Fatal("Wanted error")
		}
		testutils.AssertContains(t, err.Error(), "not found", "rpc")
		testutils.AssertEqual(t, receivedUser.Id, "")
	})

	t.Run("bad-data-entered", func(t *testing.T) {
		receivedUser, err := store.GetUserInfo(ctx, "wrong-type")
		if err == nil {
			t.Fatal("Wanted error")
		}
		testutils.AssertContains(t, err.Error(), "LocalDocument")
		testutils.AssertEqual(t, receivedUser.Id, "")
	})
}

func TestGoogleGroupRegistration(t *testing.T) {
	fsClient := NewLocalFirestoreClient()
	store := GoogleStore{FsClient: fsClient}
	user := UserInfo{
		Id:     "user-id",
		Roles:  map[string]RoleKind{"org1": RoleEditor},
		Groups: map[string][]string{"org1": {"group1"}},
	}

	ctx := context.Background()
	err := store.RegisterUser(ctx, &user)
	testutils.AssertNil(t, err)

	err = store.RegisterGroup(ctx, "user-id", "org1", "some-new-group")
	testutils.AssertNil(t, err)

	receivedUser, err := store.GetUserInfo(ctx, "user-id")
	testutils.AssertNil(t, err)
	testutils.AssertEqual(t, len(receivedUser.Groups["org1"]), 2)

	err = store.RemoveGroup(ctx, "user-id", "org1", "some-new-group")
	testutils.AssertNil(t, err)

	receivedUser, err = store.GetUserInfo(ctx, "user-id")
	testutils.AssertNil(t, err)
	testutils.AssertEqual(t, len(receivedUser.Groups["org1"]), 1)
}

func TestGoogleRegisterRole(t *testing.T) {
	store := GoogleStore{FsClient: NewLocalFirestoreClient()}
	user := UserInfo{
		Id:    "user1",
		Roles: map[string]RoleKind{"org1": RoleEditor},
	}

	ctx := context.Background()
	err := store.RegisterUser(ctx, &user)
	testutils.AssertNil(t, err)

	err = store.RegisterRole(ctx, "user1", "org1", RoleAdmin)
	testutils.AssertNil(t, err)

	receivedUser, err := store.GetUserInfo(ctx, "user1")
	testutils.AssertNil(t, err)
	testutils.AssertEqual(t, receivedUser.Roles["org1"], RoleAdmin)
}

func TestGoogleDeleteRole(t *testing.T) {
	store := GoogleStore{FsClient: NewLocalFirestoreClient()}
	user := UserInfo{
		Id:    "user1",
		Roles: map[string]RoleKind{"org1": RoleEditor},
	}

	ctx := context.Background()
	err := store.RegisterUser(ctx, &user)
	testutils.AssertNil(t, err)

	err = store.DeleteRole(ctx, "user1", "org1")
	testutils.AssertNil(t, err)

	receivedUser, err := store.GetUserInfo(ctx, "user1")
	testutils.AssertNil(t, err)

	_, hasRole := receivedUser.Roles["org1"]
	testutils.AssertEqual(t, hasRole, false)

}

func TestUniqueErrors(t *testing.T) {
	errs := []error{nil, nil}
	testutils.AssertNil(t, uniqueErrors(errs))

	myErr := errors.New("my error")
	errs = append(errs, myErr)
	errs = append(errs, myErr)
	all := uniqueErrors(errs)
	testutils.AssertEqual(t, all.Error(), "my error")
}

func TestGoogleGetUsersInOrg(t *testing.T) {
	store := GoogleStore{FsClient: NewLocalFirestoreClient()}
	ctx := context.Background()
	for i := range 6 {
		orgId := fmt.Sprintf("org%d", i%2)
		user := UserInfo{
			Id:    fmt.Sprintf("user%d", i),
			Roles: map[string]RoleKind{orgId: RoleEditor},
		}

		err := store.RegisterUser(ctx, &user)
		testutils.AssertNil(t, err)
	}

	usersInOrg, err := store.GetUsersInOrg(ctx, "org1")
	testutils.AssertNil(t, err)
	testutils.AssertEqual(t, len(usersInOrg), 3)
}

func TestGoogleResourceItemNames(t *testing.T) {
	store := GoogleStore{
		BucketClient: NewLocalBucketClient(),
		FsClient:     NewLocalFirestoreClient(),
		Config:       NewTestConfig(),
	}
	content := func(yield func(n string, b []byte) bool) {
		yield("part.pdf", []byte("content"))
	}

	ctx := context.Background()
	for i := range 3 {
		meta := MetaData{Title: fmt.Sprintf("%d my song", i)}
		err := store.Submit(ctx, "org", &meta, content)
		testutils.AssertNil(t, err)
	}

	names, err := store.ResourceItemNames(ctx, "org/2m")
	testutils.AssertNil(t, err)
	testutils.AssertEqual(t, len(names), 1)
	testutils.AssertEqual(t, names[0], "caesura-test/org/2mysong/part.pdf")
}

func TestLoadGoogleConfig(t *testing.T) {
	env := map[string]string{
		"CAESURA_BUCKET":             "my-bucket",
		"CAESURA_PROJECT_ID":         "my-project-id",
		"CAESURA_GOOGLE_ENVIRONMENT": "staging",
	}
	origValues := make(map[string]string)
	for key := range env {
		value, ok := os.LookupEnv(key)
		if ok {
			origValues[key] = value
		}
	}

	defer func() {
		for key := range env {
			orig, ok := origValues[key]
			if ok {
				os.Setenv(key, orig)
			} else {
				os.Unsetenv(key)
			}
		}
	}()

	for key, value := range env {
		os.Setenv(key, value)
	}

	config := LoadGoogleConfig()
	want := GoogleConfig{
		Bucket:      "my-bucket",
		ProjectId:   "my-project-id",
		Environment: "staging",
	}
	testutils.AssertEqual(t, *config, want)
}

func TestGetUserByEmailGoogle(t *testing.T) {
	user := UserInfo{
		Id:    "user-id",
		Email: "john@example.com",
	}

	fsClient := NewLocalFirestoreClient()
	store := GoogleStore{FsClient: fsClient}

	ctx := context.Background()
	err := store.RegisterUser(ctx, &user)
	testutils.AssertNil(t, err)

	receivedUser, err := store.UserByEmail(ctx, "john@example.com")
	testutils.AssertNil(t, err)
	testutils.AssertEqual(t, receivedUser.Id, user.Id)
}

func TestGoogleResetPassword(t *testing.T) {
	user := UserInfo{
		Id:       "user-id",
		Password: "top-secret",
	}

	fsClient := NewLocalFirestoreClient()
	store := GoogleStore{FsClient: fsClient}
	err := store.RegisterUser(context.Background(), &user)
	testutils.AssertNil(t, err)
	err = store.ResetPassword(context.Background(), "user-id", "new-top-secret-password")
	testutils.AssertNil(t, err)

	receivedUser, err := store.GetUserInfo(context.Background(), "user-id")
	testutils.AssertNil(t, err)
	testutils.AssertEqual(t, receivedUser.Password, "new-top-secret-password")
}
