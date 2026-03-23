package model

type MimeType string

const (
	ApplicationJSON MimeType = "application/json"
	PlainText       MimeType = "plain/txt"
)

type Body struct {
	Type MimeType
	Raw  string
}
