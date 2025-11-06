package pkg

import (
	"bytes"
	"fmt"
	"time"

	"github.com/davidkleiven/caesura/utils"
	"golang.org/x/crypto/bcrypt"
)

func NewLargeDemoStore() *MultiOrgInMemoryStore {
	store := NewMultiOrgInMemoryStore()
	numOrgs := 5
	num := 100

	firstNames := []string{"John", "Susan", "Allister", "Mary", "Peter"}
	lastNames := []string{"Smith", "Rodgers", "Peterson", "Anderson", "Mitchell"}
	adjectives := []string{"Bright", "Silent", "Curious", "Gentle", "Swift"}
	nouns := []string{"River", "Mountain", "Lantern", "Garden", "Bridge"}

	for orgNum := range numOrgs {
		orgId := fmt.Sprintf("OrgId%d", orgNum)
		singleOrgStore := NewInMemoryStore()
		singleOrgStore.Metadata = make([]MetaData, num)
		for i := range num {
			n := seededRand.Intn(len(adjectives))
			title := adjectives[n] + " " + nouns[n]

			n = seededRand.Intn(len(firstNames))
			composer := firstNames[n] + " " + lastNames[n]

			n = seededRand.Intn(len(firstNames))
			arranger := firstNames[n] + " " + lastNames[n]
			singleOrgStore.Metadata[i].Title = title
			singleOrgStore.Metadata[i].Composer = composer
			singleOrgStore.Metadata[i].Arranger = arranger
		}

		var pdfBuf bytes.Buffer
		PanicOnErr(CreateNPagePdf(&pdfBuf, 2))
		content := pdfBuf.Bytes()
		parts := []string{"Cornet", "Tenor"}
		for _, m := range singleOrgStore.Metadata {
			name := m.ResourceId()
			for i := range 5 {
				fname := fmt.Sprintf("%s/%s%d.pdf", name, parts[i%2], i)
				singleOrgStore.Data[fname] = content
			}
		}

		singleOrgStore.Projects = make(map[string]Project)
		for range num {
			projectDate := time.Now()
			n := seededRand.Intn(len(adjectives))
			projectTitle := adjectives[n] + " " + nouns[n]
			resourceIds := make([]string, len(singleOrgStore.Metadata))
			for j, m := range singleOrgStore.Metadata {
				resourceIds[j] = m.ResourceId()
			}
			project := Project{
				Name:        projectTitle,
				ResourceIds: resourceIds,
				CreatedAt:   projectDate,
				UpdatedAt:   projectDate,
			}
			singleOrgStore.Projects[project.Id()] = project
		}
		store.Data[orgId] = singleOrgStore
	}

	store.Users = make([]UserInfo, num)
	for i := range num {
		store.Users[i].Id = RandomInsecureID()
		n := seededRand.Intn(len(firstNames))
		store.Users[i].Name = firstNames[n] + " " + lastNames[n]
		store.Users[i].Roles = make(map[string]RoleKind)
		store.Users[i].Groups = make(map[string][]string)
		for orgId := range store.Data {
			store.Users[i].Roles[orgId] = RoleEditor
			store.Users[i].Groups[orgId] = []string{"Tenor", "Bass"}
		}
	}

	orgCounter := 0
	validSubscription := Subscription{
		Id:        "sub1",
		Expires:   time.Now().Add(time.Hour),
		MaxScores: 1000,
	}
	// Add a special user that is admin
	hash := utils.Must(bcrypt.GenerateFromPassword([]byte("demopassword"), bcrypt.DefaultCost))
	demoUser := UserInfo{
		Id:       RandomInsecureID(),
		Email:    "testuser@example.com",
		Name:     "Demo user",
		Password: string(hash),
		Roles:    make(map[string]RoleKind),
		Groups:   make(map[string][]string),
	}

	for orgId := range store.Data {
		store.Organizations = append(store.Organizations, Organization{Id: orgId, Name: fmt.Sprintf("Org %d", orgCounter)})
		orgCounter++
		store.Subscriptions[orgId] = validSubscription
		demoUser.Roles[orgId] = RoleAdmin
		demoUser.Groups[orgId] = []string{"Cornet"}
	}
	store.Users = append(store.Users, demoUser)
	return store
}
