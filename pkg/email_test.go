package pkg

import (
	"bytes"
	"context"
	"encoding/base64"
	"io"
	"iter"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/mail"
	"net/smtp"
	"strings"
	"testing"
	"time"

	"github.com/davidkleiven/caesura/testutils"
)

func buildTestEmail() (*bytes.Buffer, error) {
	email := Email{
		Sender:    "sender@example.com",
		Recipents: []string{"recipient@example.com"},
	}

	attachments := iter.Seq2[string, io.Reader](func(yield func(string, io.Reader) bool) {
		yield("test.pdf", bytes.NewReader([]byte("PDF-CONTENT-123")))
	})

	return email.Build("Test Subject", "Hello, this is a test email.", attachments)
}

func TestEmailBuild_SubjectHeader(t *testing.T) {
	msgBytes, err := buildTestEmail()
	testutils.AssertNil(t, err)

	msg, err := mail.ReadMessage(msgBytes)
	testutils.AssertNil(t, err)
	testutils.AssertEqual(t, msg.Header.Get("Subject"), "Test Subject")
}

func TestEmailBuild_MultipartBoundary(t *testing.T) {
	msgBytes, err := buildTestEmail()
	testutils.AssertNil(t, err)
	msg, err := mail.ReadMessage(msgBytes)
	testutils.AssertNil(t, err)
	contentType := msg.Header.Get("Content-Type")
	mediaType, params, err := mime.ParseMediaType(contentType)
	testutils.AssertNil(t, err)

	if !strings.HasPrefix(mediaType, "multipart/") {
		t.Fatalf("Expected multipart/*, got %s", mediaType)
	}

	if _, ok := params["boundary"]; !ok {
		t.Fatal("Missing boundary parameter in Content-Type")
	}
}

func TestEmailBuild_TextBody(t *testing.T) {
	msgBytes, err := buildTestEmail()
	testutils.AssertNil(t, err)

	msg, err := mail.ReadMessage(msgBytes)
	testutils.AssertNil(t, err)
	_, params, err := mime.ParseMediaType(msg.Header.Get("Content-Type"))
	testutils.AssertNil(t, err)
	mr := multipart.NewReader(msg.Body, params["boundary"])

	part, err := mr.NextPart()
	testutils.AssertNil(t, err)

	body, err := io.ReadAll(quotedprintable.NewReader(part))
	testutils.AssertNil(t, err)

	want := "Hello, this is a test email."
	testutils.AssertEqual(t, string(body), want)
}

func TestEmailBuild_AttachmentHeaders(t *testing.T) {
	msgBytes, err := buildTestEmail()
	testutils.AssertNil(t, err)
	msg, err := mail.ReadMessage(msgBytes)
	testutils.AssertNil(t, err)
	_, params, err := mime.ParseMediaType(msg.Header.Get("Content-Type"))
	testutils.AssertNil(t, err)
	mr := multipart.NewReader(msg.Body, params["boundary"])

	// Skip text part
	_, err = mr.NextPart()
	testutils.AssertNil(t, err)

	// Attachment part
	part, err := mr.NextPart()
	testutils.AssertNil(t, err)
	testutils.AssertContains(t, part.Header.Get("Content-Disposition"), "attachment")
	testutils.AssertEqual(t, part.FileName(), "test.pdf")
}

func TestEmailBuild_AttachmentContent(t *testing.T) {
	msgBytes, err := buildTestEmail()
	testutils.AssertNil(t, err)
	msg, err := mail.ReadMessage(msgBytes)
	testutils.AssertNil(t, err)
	_, params, err := mime.ParseMediaType(msg.Header.Get("Content-Type"))
	testutils.AssertNil(t, err)
	mr := multipart.NewReader(msg.Body, params["boundary"])

	// Skip body
	_, err = mr.NextPart()
	testutils.AssertNil(t, err)

	// Attachment
	part, err := mr.NextPart()
	testutils.AssertNil(t, err)

	encoded, err := io.ReadAll(part)
	testutils.AssertNil(t, err)
	decoded := make([]byte, base64.StdEncoding.DecodedLen(len(encoded)))
	n, err := base64.StdEncoding.Decode(decoded, bytes.ReplaceAll(encoded, []byte("\r\n"), nil))
	testutils.AssertNil(t, err)

	want := "PDF-CONTENT-123"
	testutils.AssertEqual(t, part.FileName(), "test.pdf")
	testutils.AssertEqual(t, string(decoded[:n]), want)
}

func TestBuild_AttachmentReadFails(t *testing.T) {
	email := Email{}
	body := "Hello"
	attachments := func(yield func(string, io.Reader) bool) {
		yield("file.pdf", &failingReader{})
	}

	_, err := email.Build("Subject", body, attachments)
	if err == nil {
		t.Fatalf("expected read error, got: %v", err)
	}
}

func TestPrepareEmails(t *testing.T) {
	resources := []string{"song/trumpet1.pdf", "song/clarinet2.pdf", "song/trombone3.pdf", "song/bass.pdf"}
	users := []UserInfo{
		{
			Email:  "georgine@example.com",
			Groups: map[string][]string{"0000": {"Clarinet"}},
		},
		{
			Email:  "john@example.com",
			Groups: map[string][]string{"0000": {"Trumpet"}},
		},
		{
			Email:  "peter@example.com",
			Groups: map[string][]string{"0000": {"Trumpet", "Trombone"}},
		},
		{
			Email:  "susan@example.com",
			Groups: map[string][]string{"0000": {"Clarinet"}},
		},
		{
			Email:  "hector@example.com",
			Groups: map[string][]string{"0000": {"Trombone"}},
		},
	}

	results := PrepareEmails(users, resources, "0000")
	want := []int{0, 3, 1, 2, 4}

	for i, result := range results.Emails {
		testutils.AssertEqual(t, result.Addr, users[want[i]].Email)
	}

	wantLastRequire := map[string]int{
		"song/clarinet2.pdf": 1,
		"song/trumpet1.pdf":  3,
		"song/trombone3.pdf": 4,
	}

	testutils.AssertEqual(t, len(results.LastUserRequireResource), len(wantLastRequire))

	for k, v := range results.LastUserRequireResource {
		testutils.AssertEqual(t, v, wantLastRequire[k])
	}
}

func TestUsersWithNoGroupsNotIncluded(t *testing.T) {
	resources := []string{"song/trumpet1.pdf"}
	users := []UserInfo{
		{
			Email:  "georgine@example.com",
			Groups: map[string][]string{"0000": {"Clarinet"}},
		},
	}

	for _, orgId := range []string{"0000", "0001"} {
		t.Run(orgId, func(t *testing.T) {
			results := PrepareEmails(users, resources, orgId)
			testutils.AssertEqual(t, len(results.Emails), 0)
		})
	}
}

func TestEmailOpts(t *testing.T) {
	var (
		called      bool
		recipents   []string
		emailSender string
		message     []byte
	)

	sendFn := func(addr string, auth smtp.Auth, sender string, recipent []string, msg []byte) error {
		called = true
		recipents = recipent
		emailSender = sender
		message = msg
		return nil
	}

	email := NewEmail(
		WithHost("myhost"),
		WithPort("2000"),
		WithRecipents([]string{"rec"}),
		WithSendFn(sendFn),
		WithSender("me"),
		WithAuth(smtp.PlainAuth("me", "myname", "password", "host")),
	)

	timeout, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	err := email.Send(timeout, []byte("kjd"))
	testutils.AssertNil(t, err)
	testutils.AssertEqual(t, called, true)
	testutils.AssertEqual(t, email.SmtpHost, "myhost")
	testutils.AssertEqual(t, email.SmtpPort, "2000")
	testutils.AssertEqual(t, len(recipents), 1)
	testutils.AssertEqual(t, recipents[0], "rec")
	testutils.AssertEqual(t, emailSender, "me")
	testutils.AssertEqual(t, string(message), "kjd")
}
