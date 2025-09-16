package pkg

import (
	"context"
	"reflect"
	"testing"

	"cloud.google.com/go/firestore"
	"github.com/davidkleiven/caesura/testutils"
)

func TestLocalFirestoreClientErrorOnNotMetaData(t *testing.T) {
	client := NewLocalFirestoreClient()
	err := client.Update(context.Background(), "org", "collection", "doc", []firestore.Update{{Path: "status", Value: "finished"}})
	if err == nil {
		t.Fatal("Wanted error")
	}

	testutils.AssertContains(t, err.Error(), "convert to FirestoreMetaData")
}

func TestErrorOnUnsupportedUpdate(t *testing.T) {
	client := NewLocalFirestoreClient()
	client.data["collection/org/doc"] = &FirestoreMetaData{}
	err := client.Update(context.Background(), "collection", "org", "doc", []firestore.Update{{Path: "status", Value: 2}})
	if err == nil {
		t.Fatal("Wanted error")
	}

	testutils.AssertContains(t, err.Error(), "into StoreStatus")
}

func TestAllFirestoreRecordsHasFirestoreTag(t *testing.T) {
	objs := []any{Project{}, MetaData{}, User{}, UserOrganizationLink{}}
	for _, item := range objs {
		tp := reflect.TypeOf(item)
		for i := range tp.NumField() {
			if tp.Field(i).Tag.Get("firestore") == "" {
				t.Fatalf("Object %s field %s missing 'firestore' tag", tp, tp.Field(i).Name)
			}
		}
	}
}

type pair struct {
	A string
	B int
}

func TestLocalDocument(t *testing.T) {
	obj := pair{A: "hey", B: 2}

	t.Run("valid pointer", func(t *testing.T) {
		doc := LocalDocument{data: &obj}
		var target pair
		testutils.AssertNil(t, doc.DataTo(&target))
		testutils.AssertEqual(t, obj, target)
	})

	t.Run("not a pointer", func(t *testing.T) {
		doc := LocalDocument{data: obj}
		var target pair
		err := doc.DataTo(target)
		if err == nil {
			t.Fatal("Wanted error")
		}
		testutils.AssertContains(t, err.Error(), "non-nil pointer")
	})
}

func TestLocalClientGetDocByPrefix(t *testing.T) {
	data := MetaData{Title: "My title"}
	client := NewLocalFirestoreClient()
	ctx := context.Background()
	err := client.StoreDocument(ctx, "dataset", "my-org", "meta1", &data)
	testutils.AssertNil(t, err)

	iter := client.GetDocByPrefix(ctx, "dataset", "my-org", "title", "My")
	num := 0
	for range iter {
		num += 1
	}
	testutils.AssertEqual(t, num, 1)

	num = 0
	iter = client.GetDocByPrefix(ctx, "dataset", "my-org", "title", "my")
	for range iter {
		num += 1
	}
	testutils.AssertEqual(t, num, 0)

}

func TestLocalClientGetDoc(t *testing.T) {
	data := MetaData{Title: "title"}
	client := NewLocalFirestoreClient()
	ctx := context.Background()
	err := client.StoreDocument(ctx, "dataset", "my-org", "meta1", &data)
	testutils.AssertNil(t, err)

	t.Run("error on non existing", func(t *testing.T) {
		_, err := client.GetDoc(ctx, "dataset", "my-org", "meta2")
		if err == nil {
			t.Fatal("Should fail because not found")
		}
	})

	t.Run("error on non existing", func(t *testing.T) {
		_, err := client.GetDoc(ctx, "dataset", "my-org", "meta1")
		testutils.AssertNil(t, err)
	})
}
