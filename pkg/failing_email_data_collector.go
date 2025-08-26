package pkg

import (
	"context"
	"iter"
)

type FailingEmailDataCollector struct {
	ErrItemGetter        error
	ErrResourceItemNames error
	ErrUsersInOrg        error
	ErrMetaById          error
	ResourceNames        []string
	Users                []UserInfo
}

func (f *FailingEmailDataCollector) Item(ctx context.Context, path string) ([]byte, error) {
	return []byte{}, f.ErrItemGetter
}

func (f *FailingEmailDataCollector) ResourceItemNames(ctx context.Context, resourceId string) ([]string, error) {
	return f.ResourceNames, f.ErrResourceItemNames
}

func (f *FailingEmailDataCollector) Resource(ctx context.Context, orgId string, path string) iter.Seq2[string, []byte] {
	return func(yield func(string, []byte) bool) {}
}

func (f *FailingEmailDataCollector) GetUsersInOrg(ctx context.Context, orgId string) ([]UserInfo, error) {
	return f.Users, f.ErrUsersInOrg
}

func (f *FailingEmailDataCollector) MetaById(ctx context.Context, orgId string, id string) (*MetaData, error) {
	return &MetaData{}, f.ErrMetaById
}
