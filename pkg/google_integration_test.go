package pkg

import (
	"context"
	"io"
	"os"
	"os/exec"
	"path"
	"strings"
	"testing"
	"time"

	"cloud.google.com/go/firestore"
	"cloud.google.com/go/storage"
	"github.com/davidkleiven/caesura/testutils"
)

func isCI() bool {
	return os.Getenv("CI") != ""
}

func isMainBranch() bool {
	ref := os.Getenv("GITHUB_REF")
	const prefix = "refs/heads/"
	ref = strings.TrimPrefix(ref, prefix)
	return ref == "main"
}

func isGcloudAuthenticated() bool {
	cmd := exec.Command("gcloud", "auth", "print-access-token")
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	token := strings.TrimSpace(string(output))
	return token != ""
}

func skipGoogleIntegration() bool {
	_, ok := os.LookupEnv("CAESURA_SKIP_GOOGLE_INTTEST")
	return ok
}

func checkSkipGoogleIntegration(t *testing.T) {
	if isCI() && !isMainBranch() {
		t.Skip("Test only runs on main branch to avoid uploading uncontrolled data from PR")
	} else if !isCI() && !isGcloudAuthenticated() {
		t.Skip("Local test and not authenticated to google")
	} else if skipGoogleIntegration() {
		t.Skip("Skip google integration test")
	}
}

func TestUploadToGoogle(t *testing.T) {
	checkSkipGoogleIntegration(t)
	client, err := storage.NewClient(context.Background())
	testutils.AssertNil(t, err)
	gcsClient := GCSBucketClient{client: client}

	cfg := NewTestConfig()

	fc, err := firestore.NewClient(context.Background(), cfg.ProjectId)
	testutils.AssertNil(t, err)
	defer fc.Close()

	fsClient := GoogleFirestoreClient{client: fc, environment: "test"}

	submitData := createSubmitData(&gcsClient, &fsClient)

	timeout, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	err = submitData.store.Submit(timeout, submitData.orgId, submitData.meta, submitData.data)
	testutils.AssertNil(t, err)

	// Test fetching one bucket
	content, err := gcsClient.GetObject(timeout, "caesura-test", path.Join(submitData.orgId, submitData.meta.ResourceId(), "data0.pdf"))
	testutils.AssertNil(t, err)

	contentBytes, err := io.ReadAll(content)
	testutils.AssertNil(t, err)
	if len(contentBytes) == 0 {
		t.Fatal("Content should not be empty")
	}

	// Test listing buckets matching a prefix
	iter := gcsClient.GetObjects(timeout, "caesura-test", &storage.Query{Prefix: path.Join(submitData.orgId, submitData.meta.ResourceId())})

	want := map[string]struct{}{
		path.Join(submitData.orgId, submitData.meta.ResourceId(), "data0.pdf"): {},
		path.Join(submitData.orgId, submitData.meta.ResourceId(), "data1.pdf"): {},
	}

	num := 0
	for {
		obj, err := iter.Next()
		if err != nil {
			break
		}
		num += 1
		_, ok := want[obj.Name]
		if !ok {
			t.Fatalf("%s not in %v", obj.Name, want)
		}
	}
	testutils.AssertEqual(t, num, 2)
}

func TestFireStoreFieldQuery(t *testing.T) {
	checkSkipGoogleIntegration(t)
	config := NewTestConfig()
	client, err := firestore.NewClient(context.Background(), config.ProjectId)
	testutils.AssertNil(t, err)
	defer client.Close()

	fsClient := GoogleFirestoreClient{client: client, environment: "test"}

	doc := struct {
		FirstName string `firestore:"first-name"`
		LastName  string `firestore:"last-name"`
	}{
		FirstName: "Frankie",
		LastName:  "Boy",
	}
	timeout, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err = fsClient.StoreDocument(timeout, "dataset", "org", "my-item", &doc)
	testutils.AssertNil(t, err)

	iter := fsClient.GetDocByPrefix(timeout, "dataset", "org", "first-name", "Frank")
	num := 0
	for range iter {
		num += 1
	}
	testutils.AssertEqual(t, num, 1)

	t.Run("get document by id", func(t *testing.T) {
		_, err := fsClient.GetDoc(timeout, "dataset", "org", "my-item")
		testutils.AssertNil(t, err)
	})
}

func TestRemoveResourceIdFirestore(t *testing.T) {
	checkSkipGoogleIntegration(t)
	project := Project{
		Name:        "Wurlitzer masters",
		ResourceIds: []string{"song1", "song2", "song3"},
	}
	config := NewTestConfig()
	fsClient, err := firestore.NewClient(context.Background(), config.ProjectId)
	testutils.AssertNil(t, err)
	store := GoogleStore{FsClient: &GoogleFirestoreClient{client: fsClient, environment: "test"}}
	timeout, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	orgId := "theatre-org"

	err = store.SubmitProject(timeout, orgId, &project)
	testutils.AssertNil(t, err)

	err = store.RemoveResource(timeout, orgId, project.Id(), "song2")
	testutils.AssertNil(t, nil)

	storeProject, err := store.ProjectById(timeout, orgId, project.Id())
	testutils.AssertNil(t, err)
	testutils.AssertEqual(t, len(storeProject.ResourceIds), 2)

	want := []string{"song1", "song3"}
	for i := range len(want) {
		if want[i] != storeProject.ResourceIds[i] {
			t.Fatalf("Wanted %v got %v", want, storeProject.ResourceIds)
		}
	}

}

func TestFireStoreDeleteDoc(t *testing.T) {
	checkSkipGoogleIntegration(t)
	config := NewTestConfig()
	client, err := firestore.NewClient(context.Background(), config.ProjectId)
	testutils.AssertNil(t, err)
	defer client.Close()

	fsClient := GoogleFirestoreClient{client: client, environment: "test"}

	doc := struct {
		Id string `firestore:"id"`
	}{
		Id: "id",
	}
	timeout, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err = fsClient.StoreDocument(timeout, "dataset", "org", "my-item", &doc)
	testutils.AssertNil(t, err)

	_, err = fsClient.GetDoc(timeout, "dataset", "org", "my-item")
	testutils.AssertNil(t, err)

	err = fsClient.DeleteDoc(timeout, "dataset", "org", "my-item")
	testutils.AssertNil(t, err)

	_, err = fsClient.GetDoc(timeout, "dataset", "org", "my-item")
	if err == nil {
		t.Fatal("Wanted error because document should not exist")
	}
}
