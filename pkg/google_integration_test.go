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

func TestUploadToGoogle(t *testing.T) {
	if isCI() && !isMainBranch() {
		t.Skip("Test only runs on main branch to avoid uploading uncontrolled data from PR")
		return
	} else if !isCI() && !isGcloudAuthenticated() {
		t.Skip("Local test and not authenticated to google")
		return
	}
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
