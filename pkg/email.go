package pkg

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"iter"
	"math"
	"mime/multipart"
	"mime/quotedprintable"
	"net/smtp"
	"net/textproto"
	"slices"
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

type PreparedEmail struct {
	Addr          string
	ResourceNames []string
}

type PreparedEmails struct {
	Emails                  []PreparedEmail
	LastUserRequireResource map[string]int
}

// PrepareEmails assigns resource names to each email address
// The users are ordered such that users with similar attachments
// are ordered next to each other
func PrepareEmails(users []UserInfo, resourceNames []string, orgId string) *PreparedEmails {
	if len(users) == 0 || len(resourceNames) == 0 {
		return &PreparedEmails{
			Emails:                  []PreparedEmail{},
			LastUserRequireResource: make(map[string]int),
		}
	}
	desc := make([][]int, len(users))
	prepEmail := make([]PreparedEmail, len(users))
	lastUser := make(map[string]int)

	// Create a descriptor describing which resource various users should have
	for userNo, user := range users {
		groups, ok := user.Groups[orgId]
		if !ok {
			continue
		}
		prepEmail[userNo].Addr = user.Email
		for _, group := range groups {
			for i, name := range resourceNames {
				if strings.Contains(strings.ToLower(name), strings.ToLower(group)) {
					desc[userNo] = append(desc[userNo], i)
					lastUser[name] = userNo
					prepEmail[userNo].ResourceNames = append(prepEmail[userNo].ResourceNames, name)
				}
			}
		}
	}

	simMatrix := NewLowerTriangularMatrix(len(users))
	for i := range len(users) {
		for j := i + 1; j < len(users); j++ {
			simMatrix.Set(i, j, similarity(desc[i], desc[j]))
		}
	}
	ordered := greedyOrderBySimilarity(simMatrix)

	orderedPreppedEmail := make([]PreparedEmail, len(users))
	for i, newIdx := range ordered {
		orderedPreppedEmail[i] = prepEmail[newIdx]
	}

	invMapping := make([]int, len(ordered))
	for i, idx := range ordered {
		invMapping[idx] = i
	}

	// Update last used
	for k, v := range lastUser {
		lastUser[k] = invMapping[v]
	}

	return &PreparedEmails{
		Emails: slices.DeleteFunc(orderedPreppedEmail, func(p PreparedEmail) bool {
			return len(p.ResourceNames) == 0
		}),
		LastUserRequireResource: lastUser,
	}
}

func similarity(idx1, idx2 []int) float64 {
	if len(idx1) == 0 || len(idx2) == 0 {
		return 0.0
	}
	innerProd := 0.0
	for _, i1 := range idx1 {
		for _, i2 := range idx2 {
			if i1 == i2 {
				innerProd += 1.0
			}
		}
	}

	l1 := math.Sqrt(float64(len(idx1)))
	l2 := math.Sqrt(float64(len(idx2)))
	return innerProd / (l1 * l2)
}

type LowerTriangularMatrix struct {
	data []float64
	N    int
}

func (l *LowerTriangularMatrix) index(i, j int) int {
	if i < j {
		i, j = j, i
	}
	return i*(i+1)/2 + j
}

func (l *LowerTriangularMatrix) At(i, j int) float64 {
	return l.data[l.index(i, j)]
}

func (l *LowerTriangularMatrix) Set(i, j int, v float64) {
	l.data[l.index(i, j)] = v
}

func NewLowerTriangularMatrix(n int) *LowerTriangularMatrix {
	return &LowerTriangularMatrix{
		N:    n,
		data: make([]float64, n*(n+1)/2),
	}
}

// orders items by similarity
func greedyOrderBySimilarity(sim *LowerTriangularMatrix) []int {
	result := make([]int, sim.N)
	result[0] = 0

	visited := make(map[int]struct{})
	visited[0] = struct{}{}
	for i := 1; i < sim.N; i++ {
		maxValue := math.Inf(-1)
		closest := -1
		for j := range sim.N {
			_, seen := visited[j]
			if !seen {
				similarity := math.Abs(sim.At(result[i-1], j))
				if similarity > maxValue {
					closest = j
					maxValue = similarity
				}
			}
		}
		result[i] = closest
		visited[closest] = struct{}{}
	}
	return result
}

type EmailDataCollector interface {
	UserInOrgGetter
	ResourceGetter
	ResourceItemNames(ctx context.Context, resourceId string) ([]string, error)
	ItemGetter
}

func NoOpSendFunc(string, smtp.Auth, string, []string, []byte) error {
	return nil
}
