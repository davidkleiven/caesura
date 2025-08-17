package pkg

import (
	"bytes"
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
		t.Errorf("Expected multipart/*, got %s", mediaType)
	}

	if _, ok := params["boundary"]; !ok {
		t.Error("Missing boundary parameter in Content-Type")
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
		t.Errorf("expected read error, got: %v", err)
	}
}
