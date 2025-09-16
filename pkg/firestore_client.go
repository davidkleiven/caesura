package pkg

import (
	"context"
	"errors"
	"path"

	"cloud.google.com/go/firestore"
)

type FirestoreClient interface {
	StoreDocument(ctx context.Context, dataset, orgId, itemId string, data any) error
	Update(ctx context.Context, dataset, orgId, itemId string, update []firestore.Update) error
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

type LocalFirestoreClient struct {
	data map[string]any
}

func (l *LocalFirestoreClient) StoreDocument(ctx context.Context, dataset, orgId, itemId string, data any) error {
	l.data[path.Join(dataset, orgId, itemId)] = data
	return nil
}

func NewLocalFirestoreClient() *LocalFirestoreClient {
	return &LocalFirestoreClient{
		data: make(map[string]any),
	}
}

func (l *LocalFirestoreClient) Update(ctx context.Context, dataset, orgId, itemId string, update []firestore.Update) error {
	location := path.Join(dataset, orgId, itemId)
	item, ok := l.data[location].(*MetaData)
	if !ok {
		return errors.New("could not convert to MetaData")
	}
	for _, u := range update {
		if u.Path == "status" {
			val, ok := u.Value.(StoreStatus)
			if !ok {
				return errors.New("could not convert status into StoreStatus")
			}
			item.Status = val
			l.data[location] = item
		}
	}
	return nil
}
