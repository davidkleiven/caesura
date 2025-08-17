//go:build integration
// +build integration

package pkg

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"slices"
	"testing"
	"time"

	"github.com/davidkleiven/caesura/testutils"
	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
)

type EmailAddress struct {
	Name    string `json:"Name"`
	Address string `json:"Address"`
}

type Message struct {
	ID          string         `json:"ID"`
	MessageID   string         `json:"MessageID"`
	Read        bool           `json:"Read"`
	From        EmailAddress   `json:"From"`
	To          []EmailAddress `json:"To"`
	Cc          []EmailAddress `json:"Cc"`
	Bcc         []EmailAddress `json:"Bcc"`
	ReplyTo     []EmailAddress `json:"ReplyTo"`
	Subject     string         `json:"Subject"`
	Created     string         `json:"Created"` // ISO8601 timestamp string
	Username    string         `json:"Username"`
	Tags        []string       `json:"Tags"`
	Size        int            `json:"Size"`
	Attachments int            `json:"Attachments"`
	Snippet     string         `json:"Snippet"`
}

type MessagesResponse struct {
	Total          int       `json:"total"`
	Unread         int       `json:"unread"`
	Count          int       `json:"count"`
	MessagesCount  int       `json:"messages_count"`
	MessagesUnread int       `json:"messages_unread"`
	Start          int       `json:"start"`
	Tags           []string  `json:"tags"`
	Messages       []Message `json:"messages"`
}

type Attachment struct {
	PartId    string `json:"PartID"`
	FileName  string `json:"FileName"`
	Type      string `json:"ContentType"`
	Size      int    `json:"Size"`
	ContentID string `json:"ContentID"`
}

type MessageDetail struct {
	ID          string         `json:"ID"`
	From        EmailAddress   `json:"From"`
	To          []EmailAddress `json:"To"`
	Subject     string         `json:"Subject"`
	Body        string         `json:"Body"`        // Might be plain text or HTML
	Attachments []Attachment   `json:"Attachments"` // Attachments info
}

func TestSendEmail(t *testing.T) {
	email := NewEmail(
		WithSender("sender@example.com"),
		WithRecipents([]string{"recipient@example.com"}),
		WithHost("127.0.0.1"),
		WithPort("1025"),
	)

	var pdf1 bytes.Buffer
	testutils.AssertNil(t, CreateNPagePdf(&pdf1, 2))

	var pdf2 bytes.Buffer
	testutils.AssertNil(t, CreateNPagePdf(&pdf2, 5))

	iter := func(yield func(name string, content io.Reader) bool) {
		for i, item := range []bytes.Buffer{pdf1, pdf2} {
			name := fmt.Sprintf("part%d.pdf", i)
			if !yield(name, &item) {
				return
			}
		}
	}
	msg, err := email.Build("Update", "Please see attached updates", iter)
	testutils.AssertNil(t, err)
	timeout, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	// Clear messages
	defer func() {
		req, err := http.NewRequest("DELETE", "http://localhost:8025/api/v1/messages", nil)
		if err != nil {
			log.Fatal(err)
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			log.Fatal(err)
		}
		defer resp.Body.Close()
		fmt.Println("Clear emails status:", resp.Status)
	}()

	err = email.Send(timeout, msg.Bytes())
	testutils.AssertNil(t, err)

	resp, err := http.Get("http://localhost:8025/api/v1/messages")
	testutils.AssertNil(t, err)
	testutils.AssertEqual(t, resp.StatusCode, 200)

	var msgs MessagesResponse
	testutils.AssertNil(t, json.NewDecoder(resp.Body).Decode(&msgs))
	if len(msgs.Messages) == 0 {
		t.Fatalf("No messages received")
	}

	latest := msgs.Messages[0]
	testutils.AssertEqual(t, email.Recipents[0], latest.To[0].Address)
	testutils.AssertEqual(t, email.Sender, latest.From.Address)
	testutils.AssertEqual(t, latest.Subject, "Update")
	testutils.AssertEqual(t, latest.Attachments, 2)

	resp, err = http.Get("http://localhost:8025/api/v1/message/" + latest.ID)
	testutils.AssertNil(t, err)

	var detail MessageDetail
	testutils.AssertNil(t, json.NewDecoder(resp.Body).Decode(&detail))

	numPages := make([]int, 2)
	conf := model.NewDefaultConfiguration()
	for i, att := range detail.Attachments {
		testutils.AssertEqual(t, att.Type, "application/pdf")

		resp, err := http.Get("http://localhost:8025/api/v1/message/" + latest.ID + "/part/" + att.PartId)
		testutils.AssertNil(t, err)

		body, err := io.ReadAll(resp.Body)
		defer resp.Body.Close()
		testutils.AssertNil(t, err)

		pdfCtx, err := api.ReadValidateAndOptimize(bytes.NewReader(body), conf)
		testutils.AssertNil(t, err)
		numPages[i] = pdfCtx.PageCount
	}

	wantPageCounts := []int{2, 5}
	if slices.Compare(numPages, wantPageCounts) != 0 {
		t.Fatalf("Wanted %v got %v", wantPageCounts, numPages)
	}
}
