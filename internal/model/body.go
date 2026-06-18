package model

type MimeType string

const (
	ApplicationJSON MimeType = "application/json"
	PlainText       MimeType = "text/plain"
)

type Body struct {
	Type MimeType
	Raw  string
}
