package pkg

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"log/slog"
	"path"
	"reflect"
	"slices"
	"strings"
	"sync"
	"time"

	"cloud.google.com/go/firestore"
	"google.golang.org/api/iterator"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const maxUtf8 = "\uf8ff"

type FirestoreClient interface {
	StoreDocument(ctx context.Context, dataset, orgId, itemId string, data any) error
	Update(ctx context.Context, dataset, orgId, itemId string, update []firestore.Update) error
	GetDocByPrefix(ctx context.Context, dataset, orgId, field, prefix string) iter.Seq[Document]
	GetDoc(ctx context.Context, dataset, orgId, itemId string) (Document, error)
	DeleteDoc(ctx context.Context, dataset, collection, item string) error
}

type Document interface {
	DataTo(obj any) error
}

type GoogleFirestoreClient struct {
	client      *firestore.Client
	environment string
}

func (g *GoogleFirestoreClient) StoreDocument(ctx context.Context, dataset, orgId, itemId string, data any) error {
	_, err := g.client.Collection(g.environment).Doc(dataset).Collection(orgId).Doc(itemId).Set(ctx, data)
	return err
}

func (g *GoogleFirestoreClient) Update(ctx context.Context, dataset, orgId, itemId string, update []firestore.Update) error {
	_, err := g.client.Collection(g.environment).Doc(dataset).Collection(orgId).Doc(itemId).Update(ctx, update)
	return err
}

func (g *GoogleFirestoreClient) GetDocByPrefix(ctx context.Context, dataset, orgId, field, prefix string) iter.Seq[Document] {
	docIter := g.client.Collection(g.environment).Doc(dataset).Collection(orgId).
		Where(field, ">=", prefix).
		Where(field, "<", prefix+maxUtf8).
		Documents(ctx)

	return func(yield func(doc Document) bool) {
		for {
			doc, err := docIter.Next()
			if err != nil {
				logOnErrorNotDone(err)
				break
			}
			if !yield(doc) {
				return
			}
		}
	}
}

func (g *GoogleFirestoreClient) GetDoc(ctx context.Context, dataset, orgId, itemId string) (Document, error) {
	return g.client.Collection(g.environment).Doc(dataset).Collection(orgId).Doc(itemId).Get(ctx)
}

func (g *GoogleFirestoreClient) DeleteDoc(ctx context.Context, dataset, collection, itemId string) error {
	_, err := g.client.Collection(g.environment).Doc(dataset).Collection(collection).Doc(itemId).Delete(ctx)
	return err
}

func logOnErrorNotDone(err error) {
	if !errors.Is(err, iterator.Done) {
		slog.Error("Error occured when iterating over document", "error", err)
	}
}

type LocalFirestoreClient struct {
	mu   sync.Mutex
	data map[string]any
}

func (l *LocalFirestoreClient) StoreDocument(ctx context.Context, dataset, orgId, itemId string, data any) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.data[path.Join(dataset, orgId, itemId)] = data
	return nil
}

func (l *LocalFirestoreClient) GetDoc(ctx context.Context, dataset, orgId, itemId string) (Document, error) {
	loc := path.Join(dataset, orgId, itemId)
	item, ok := l.data[loc]
	if !ok {
		return nil, status.Errorf(codes.NotFound, "%s/%s not found", dataset, loc)
	}

	return &LocalDocument{data: item}, nil
}

func (l *LocalFirestoreClient) DeleteDoc(ctx context.Context, dataset, collection, itemId string) error {
	loc := path.Join(dataset, collection, itemId)
	delete(l.data, loc)
	return nil
}

func NewLocalFirestoreClient() *LocalFirestoreClient {
	return &LocalFirestoreClient{
		data: make(map[string]any),
	}
}

// Update emulates a firestore update. But some if the update commands utilize unexported field, in which case
// it tries to update 'something' but not nessecarily the exact values
func (l *LocalFirestoreClient) Update(ctx context.Context, dataset, orgId, itemId string, update []firestore.Update) error {
	location := path.Join(dataset, orgId, itemId)
	for _, u := range update {
		switch u.Path {
		case "status":
			item, ok := l.data[location].(*FirestoreMetaData)
			if !ok {
				return errors.New("could not convert to FirestoreMetaData")
			}
			val, ok := u.Value.(StoreStatus)
			if !ok {
				return errors.New("could not convert status into StoreStatus")
			}
			item.Status = val
			l.data[location] = item
		case "resource_ids":
			item, ok := l.data[location].(*FirestoreProject)
			if !ok {
				return errors.New("could not convert to fire store project")
			}
			slog.Warn("LocalFirebase client always removes the last item")
			item.ResourceIds = item.ResourceIds[:len(item.ResourceIds)-1]
			l.data[location] = item
		case "deleted":
			item, ok := l.data[location].(*Organization)
			if !ok {
				return errors.New("could not convert item to organization")
			}
			value, ok := u.Value.(bool)
			if !ok {
				return errors.New("could not convert value to 'bool'")
			}
			item.Deleted = value
			l.data[location] = item
		case "groups":
			item, ok := l.data[location].(UserOrganizationLink)
			if !ok {
				return errors.New("could not convert to 'UserOrganizationLink'")
			}

			updateName := reflect.TypeOf(u.Value).Name()
			switch updateName {
			case "arrayUnion":
				item.Groups = append(item.Groups, "new-group")
			case "arrayRemove":
				item.Groups = slices.DeleteFunc(item.Groups, func(n string) bool { return n == "new-group" })
			default:
				return fmt.Errorf("Unknown name %s", updateName)
			}
			l.data[location] = item
		case "role":
			item, ok := l.data[location].(UserOrganizationLink)
			if !ok {
				return errors.New("could not convert to 'UserOrganizationLink'")
			}

			role, ok := u.Value.(RoleKind)
			if !ok {
				return errors.New("could not convert value to 'RoleKind'")
			}
			item.Role = role
			l.data[location] = item
		case "password":
			item := l.data[location].(User)
			item.Password = u.Value.(string)
			l.data[location] = item
		case "updated_at":
			item := l.data[location].(*FirestoreProject)
			item.UpdatedAt = u.Value.(time.Time)
			l.data[location] = item
		}
	}
	return nil
}

func (l *LocalFirestoreClient) GetDocByPrefix(ctx context.Context, dataset, orgId, field, prefix string) iter.Seq[Document] {
	pathPrefix := path.Join(dataset, orgId)
	return func(yield func(doc Document) bool) {
		for location, data := range l.data {
			if strings.HasPrefix(location, pathPrefix) {
				val := reflect.ValueOf(data)
				if val.Kind() == reflect.Ptr {
					val = val.Elem()
				}
				t := val.Type()
				for i := range val.NumField() {
					ft := t.Field(i)
					firestoreTag := ft.Tag.Get("firestore")
					if field == firestoreTag {
						fv := val.Field(i)
						content, ok := fv.Interface().(string)
						if ok && strings.HasPrefix(content, prefix) {
							doc := LocalDocument{data: data}
							if !yield(&doc) {
								return
							}
						}
					}
				}
			}
		}
	}
}

type LocalDocument struct {
	data any
}

func (l *LocalDocument) DataTo(other any) error {
	if l.data == nil {
		return fmt.Errorf("LocalDocument.data is nil")
	}

	otherVal := reflect.ValueOf(other)
	if otherVal.Kind() != reflect.Ptr || otherVal.IsNil() {
		return fmt.Errorf("other must be a non-nil pointer")
	}

	srcVal := reflect.ValueOf(l.data)
	if srcVal.Kind() == reflect.Ptr {
		srcVal = srcVal.Elem()
	}

	dstVal := otherVal.Elem()
	dstType := dstVal.Type()
	for i := range dstVal.NumField() {
		dstField := dstVal.Field(i)
		dstStructField := dstType.Field(i)
		srcField := srcVal.FieldByName(dstStructField.Name)
		if srcField.IsValid() && srcField.Type().AssignableTo(dstField.Type()) {
			dstField.Set(srcField)
		}
	}
	return nil
}
