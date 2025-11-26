package pkg

import (
	"context"
	"errors"
	"fmt"
	"io"
	"iter"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"cloud.google.com/go/firestore"
	"cloud.google.com/go/storage"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	metaDataCollection     = "metadata"
	projectCollection      = "projects"
	subscriptionCollection = "subscriptions"
	organizationCollection = "organizations"
	organizationInfo       = "info"
	userCollection         = "users"
	userInfoDoc            = "info"
	userOrgLinkDoc         = "userOrganizationLinks"
)

type GoogleConfig struct {
	Bucket      string `yaml:"bucket" env:"CAESURA_BUCKET"`
	ProjectId   string `yaml:"projectId" env:"CAESURA_PROJECT_ID"`
	Environment string `yaml:"environment" env:"CAESURA_GOOGLE_ENVIRONMENT"`
}

func NewTestConfig() *GoogleConfig {
	return &GoogleConfig{
		Bucket:      "caesura-test",
		ProjectId:   "caesura-466820",
		Environment: "test",
	}
}

func LoadGoogleConfig() *GoogleConfig {
	config := NewTestConfig()
	return OverrideFromEnv(config, os.LookupEnv)
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

func (g *GoogleStore) MetaById(ctx context.Context, orgId, metaId string) (*MetaData, error) {
	doc, err := g.FsClient.GetDoc(ctx, metaDataCollection, orgId, metaId)
	var meta MetaData
	if err != nil {
		return &meta, err
	}
	err = doc.DataTo(&meta)
	return &meta, err
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
		{
			Path:  "updated_at",
			Value: time.Now(),
		},
	}
	return g.FsClient.Update(ctx, projectCollection, orgId, projectId, update)
}

func (g *GoogleStore) Resource(ctx context.Context, orgId string, path string) iter.Seq2[string, []byte] {
	query := storage.Query{Prefix: filepath.Join(orgId, path)}
	objects := g.BucketClient.GetObjects(ctx, g.Config.Bucket, &query)
	return func(yield func(name string, content []byte) bool) {
		for {
			objAttr, err := objects.Next()
			if err != nil {
				return
			}
			content, err := g.BucketClient.GetObject(ctx, objAttr.Bucket, objAttr.Name)
			contentBytes, err := io.ReadAll(content)
			content.Close()

			if err != nil {
				continue
			}

			resourceName := filepath.Base(objAttr.Name)
			if !yield(resourceName, contentBytes) {
				return
			}
		}
	}
}
func (g *GoogleStore) Item(ctx context.Context, path string) ([]byte, error) {
	content, err := g.BucketClient.GetObject(ctx, g.Config.Bucket, path)
	if err != nil {
		return []byte{}, err
	}
	defer content.Close()
	return io.ReadAll(content)
}

func (g *GoogleStore) StoreSubscription(ctx context.Context, stripeId string, subscription *Subscription) error {
	collector := NewValidCollector[Organization]()
	for item := range g.FsClient.GetDocByPrefix(ctx, organizationCollection, organizationInfo, "stripeId", stripeId) {
		collector.Push(item)
	}

	if len(collector.Items) == 0 {
		return fmt.Errorf("Could not find any organization for stripe id %s: %w", stripeId, ErrOrganizationNotFound)
	}
	orgId := collector.Items[0].Id
	return g.FsClient.StoreDocument(ctx, organizationCollection, subscriptionCollection, orgId, subscription)
}

func (g *GoogleStore) GetSubscription(ctx context.Context, orgId string) (*Subscription, error) {
	doc, err := g.FsClient.GetDoc(ctx, organizationCollection, subscriptionCollection, orgId)
	var sub Subscription
	if err != nil {
		return &sub, errors.Join(ErrSubscriptionNotFound, err)
	}
	err = doc.DataTo(&sub)
	return &sub, err
}

func (g *GoogleStore) RegisterOrganization(ctx context.Context, org *Organization) error {
	return g.FsClient.StoreDocument(ctx, organizationCollection, organizationInfo, org.Id, org)
}

func (g *GoogleStore) GetOrganization(ctx context.Context, orgId string) (Organization, error) {
	var org Organization
	doc, err := g.FsClient.GetDoc(ctx, organizationCollection, organizationInfo, orgId)
	if err != nil {
		return org, err
	}
	err = doc.DataTo(&org)
	return org, err
}

func (g *GoogleStore) DeleteOrganization(ctx context.Context, orgId string) error {
	return g.FsClient.Update(
		ctx,
		organizationCollection,
		organizationInfo,
		orgId,
		[]firestore.Update{{Path: "deleted", Value: true}})
}

func (g *GoogleStore) RegisterUser(ctx context.Context, userInfo *UserInfo) error {
	flatUser := userInfo.ToFlat()
	group, ctx := errgroup.WithContext(ctx)
	group.Go(func() error {
		return g.FsClient.StoreDocument(ctx, userCollection, userInfoDoc, flatUser.User.Id, flatUser.User)
	})

	for i := range flatUser.UserOrgLinks {
		link := flatUser.UserOrgLinks[i]
		group.Go(func() error {
			lId := linkId(link.UserId, link.OrgId)
			return g.FsClient.StoreDocument(ctx, userCollection, userOrgLinkDoc, lId, link)
		})
	}
	return group.Wait()
}

func (g *GoogleStore) GetUserInfo(ctx context.Context, userId string) (*UserInfo, error) {
	if userId == "" {
		return &UserInfo{}, fmt.Errorf("Empty userId provided: %w", ErrUserNotFound)
	}

	doc, err := g.FsClient.GetDoc(ctx, userCollection, userInfoDoc, userId)
	if err != nil && status.Code(err) == codes.NotFound {
		return &UserInfo{}, errors.Join(ErrUserNotFound, err)
	} else if err != nil {
		return &UserInfo{}, fmt.Errorf("Could not get document %w", err)
	}

	var user User
	if err := doc.DataTo(&user); err != nil {
		return &UserInfo{}, err
	}

	collector := NewValidCollector[UserOrganizationLink]()
	for item := range g.FsClient.GetDocByPrefix(ctx, userCollection, userOrgLinkDoc, "userId", user.Id) {
		collector.Push(item)
	}
	flat := FlatUser{
		User:         user,
		UserOrgLinks: collector.Items,
	}
	return NewUserFromFlat(&flat), collector.Err
}

func (g *GoogleStore) RegisterGroup(ctx context.Context, userId, orgId, group string) error {
	return g.FsClient.Update(
		ctx,
		userCollection,
		userOrgLinkDoc,
		linkId(userId, orgId),
		[]firestore.Update{{Path: "groups", Value: firestore.ArrayUnion(group)}},
	)
}

func (g *GoogleStore) RemoveGroup(ctx context.Context, userId, orgId, group string) error {
	return g.FsClient.Update(
		ctx,
		userCollection,
		userOrgLinkDoc,
		linkId(userId, orgId),
		[]firestore.Update{{Path: "groups", Value: firestore.ArrayRemove(group)}},
	)
}

func (g *GoogleStore) RegisterRole(ctx context.Context, userId string, organizationId string, role RoleKind) error {
	docId := linkId(userId, organizationId)
	err := g.FsClient.Update(
		ctx,
		userCollection,
		userOrgLinkDoc,
		docId,
		[]firestore.Update{{Path: "role", Value: role}},
	)

	if err != nil && status.Code(err) == codes.NotFound {
		slog.InfoContext(ctx, "Tried to update role before a link to organization was creaated. Establishing link...")
		userOrgLink := UserOrganizationLink{
			UserId: userId,
			OrgId:  organizationId,
			Role:   role,
		}
		err = g.FsClient.StoreDocument(ctx, userCollection, userOrgLinkDoc, docId, userOrgLink)
	}
	return err
}

func (g *GoogleStore) DeleteRole(ctx context.Context, userId, orgId string) error {
	return g.FsClient.DeleteDoc(ctx, userCollection, userOrgLinkDoc, linkId(userId, orgId))
}

func (g *GoogleStore) GetUsersInOrg(ctx context.Context, orgId string) ([]UserInfo, error) {
	collector := NewValidCollector[UserOrganizationLink]()
	for doc := range g.FsClient.GetDocByPrefix(ctx, userCollection, userOrgLinkDoc, "orgId", orgId) {
		collector.Push(doc)
	}

	users := make([]UserInfo, len(collector.Items))
	errors := make([]error, len(collector.Items))
	var wg sync.WaitGroup
	wg.Add(len(users))
	for i, link := range collector.Items {
		idx, userId := i, link.UserId
		go func() {
			defer wg.Done()

			u, err := g.GetUserInfo(ctx, userId)
			if err != nil {
				errors[idx] = err
			} else {
				users[idx] = *u
			}
		}()
	}
	wg.Wait()
	return users, uniqueErrors(errors)
}

func (g *GoogleStore) ResourceItemNames(ctx context.Context, resourceId string) ([]string, error) {
	objList := g.BucketClient.GetObjects(ctx, g.Config.Bucket, &storage.Query{Prefix: resourceId})
	names := []string{}
	for {
		item, err := objList.Next()
		if err != nil {
			return names, nil
		}
		names = append(names, item.Name)
	}
}

func (g *GoogleStore) UserByEmail(ctx context.Context, email string) (UserInfo, error) {
	var user User
	for doc := range g.FsClient.GetDocByPrefix(ctx, userCollection, userInfoDoc, "email", email) {
		doc.DataTo(&user)
		break
	}
	u, err := g.GetUserInfo(ctx, user.Id)
	return *u, err
}

// ResetPassword resets the users password
// Note that the password should be a hashed version of the password using
// a cryptographically safe hash method
func (g *GoogleStore) ResetPassword(ctx context.Context, userId, password string) error {
	return g.FsClient.Update(
		ctx,
		userCollection,
		userInfoDoc,
		userId,
		[]firestore.Update{{Path: "password", Value: password}},
	)

}

func uniqueErrors(possibleErrors []error) error {
	errs := make(map[error]struct{})
	for _, err := range possibleErrors {
		if err != nil {
			_, seen := errs[err]
			if !seen {
				errs[err] = struct{}{}
			}
		}
	}

	uniqueErrors := make([]error, 0, len(errs))
	for e := range errs {
		uniqueErrors = append(uniqueErrors, e)
	}
	return errors.Join(uniqueErrors...)
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

func linkId(userId, orgId string) string {
	return userId + "-" + orgId
}
