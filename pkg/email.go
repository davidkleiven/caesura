package pkg

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"iter"
	"mime/multipart"
	"mime/quotedprintable"
	"net/smtp"
	"net/textproto"
	"strings"
)

type SendFunc func(string, smtp.Auth, string, []string, []byte) error

type Email struct {
	Recipents []string
	Sender    string
	SmtpHost  string
	SmtpPort  string
	SmtpAuth  smtp.Auth
	SendFn    SendFunc
}

type EmailOpt func(*Email)

func NewEmail(opts ...EmailOpt) *Email {
	mail := Email{
		SendFn: smtp.SendMail,
	}
	for _, opt := range opts {
		opt(&mail)
	}
	return &mail
}

func WithRecipents(recipents []string) EmailOpt {
	return func(e *Email) {
		e.Recipents = recipents
	}
}

func WithSender(sender string) EmailOpt {
	return func(e *Email) {
		e.Sender = sender
	}
}

func WithHost(host string) EmailOpt {
	return func(e *Email) {
		e.SmtpHost = host
	}
}

func WithPort(port string) EmailOpt {
	return func(e *Email) {
		e.SmtpPort = port
	}
}

func WithAuth(auth smtp.Auth) EmailOpt {
	return func(e *Email) {
		e.SmtpAuth = auth
	}
}

func WithSendFn(fn SendFunc) EmailOpt {
	return func(e *Email) {
		e.SendFn = fn
	}
}

const MIMEBase64MaxLineLength = 76

func (e *Email) Build(subject string, body string, attachments iter.Seq2[string, io.Reader]) (*bytes.Buffer, error) {
	var msg bytes.Buffer
	boundary := "caesura-mixed-boundary"
	headers := map[string]string{
		"From":         e.Sender,
		"To":           strings.Join(e.Recipents, ","),
		"Subject":      subject,
		"MIME-Version": "1.0",
		"Content-Type": fmt.Sprintf("multipart/mixed; boundary=%q", boundary),
	}

	for k, v := range headers {
		fmt.Fprintf(&msg, "%s: %s\r\n", k, v)
	}
	msg.WriteString("\r\n")

	writer := multipart.NewWriter(&msg)
	defer writer.Close()

	writer.SetBoundary(boundary)
	textPartHeader := make(textproto.MIMEHeader)
	textPartHeader.Set("Content-Type", "text/plain; charset=utf-8")
	textPartHeader.Set("Content-Transfer-Encoding", "quoted-printable")
	textPart, err := writer.CreatePart(textPartHeader)
	if err != nil {
		return &msg, err
	}
	qp := quotedprintable.NewWriter(textPart)
	_, err = qp.Write([]byte(body))
	qp.Close()
	if err != nil {
		return &msg, err
	}

	for name, contentReader := range attachments {
		attachmentHeader := make(textproto.MIMEHeader)
		attachmentHeader.Set("Content-Type", fmt.Sprintf("application/pdf; name=%q", name))
		attachmentHeader.Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", name))
		attachmentHeader.Set("Content-Transfer-Encoding", "base64")

		attachmentPart, err := writer.CreatePart(attachmentHeader)
		if err != nil {
			return &msg, err
		}

		content, err := io.ReadAll(contentReader)
		if err != nil {
			return &msg, err
		}
		encoded := make([]byte, base64.StdEncoding.EncodedLen(len(content)))
		base64.StdEncoding.Encode(encoded, content)

		// Write in 76-char lines per MIME spec
		for i := 0; i < len(encoded); i += MIMEBase64MaxLineLength {
			end := min(i+MIMEBase64MaxLineLength, len(encoded))
			attachmentPart.Write(encoded[i:end])
			attachmentPart.Write([]byte("\r\n"))
		}
	}
	return &msg, nil
}

func (e *Email) Send(ctx context.Context, msg []byte) error {
	addr := fmt.Sprintf("%s:%s", e.SmtpHost, e.SmtpPort)
	done := make(chan error, 1)
	go func() {
		done <- e.SendFn(addr, e.SmtpAuth, e.Sender, e.Recipents, msg)
	}()

	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}
