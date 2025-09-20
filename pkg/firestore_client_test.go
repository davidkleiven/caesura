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

	testutils.AssertContains(t, err.Error(), "convert to MetaData")
}

func TestErrorOnUnsupportedUpdate(t *testing.T) {
	client := NewLocalFirestoreClient()
	client.data["collection/org/doc"] = &MetaData{}
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
