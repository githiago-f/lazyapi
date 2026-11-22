package store

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/githiago-f/lazyapi/internal/model"
)

const contractSpec = `openapi: 3.0.0
info: {title: t, version: "1"}
paths:
  /users/{id}:
    get:
      parameters:
        - {in: path, name: id, required: true, schema: {type: string}}
      responses:
        "200":
          description: OK
          content:
            application/json:
              schema:
                type: object
                required: [id, name]
                properties:
                  id: {type: integer}
                  name: {type: string}
        "404":
          description: Not found
`

func loadContractSpec(t *testing.T) model.OpenAPIRef {
	t.Helper()
	p := filepath.Join(t.TempDir(), "spec.yml")
	if err := os.WriteFile(p, []byte(contractSpec), 0o644); err != nil {
		t.Fatal(err)
	}
	return model.OpenAPIRef{FilePath: p, Path: "/users/{id}", Method: "get"}
}

func TestValidateResponse(t *testing.T) {
	ref := loadContractSpec(t)
	spec, err := ParseSpec(ref.FilePath)
	if err != nil {
		t.Fatal(err)
	}
	jsonHeader := http.Header{"Content-Type": {"application/json; charset=utf-8"}}

	if err := ValidateResponse(spec, ref, 200, jsonHeader, []byte(`{"id":1,"name":"ana"}`)); err != nil {
		t.Errorf("valid body rejected: %v", err)
	}

	err = ValidateResponse(spec, ref, 200, jsonHeader, []byte(`{"id":"x"}`))
	if err == nil {
		t.Fatal("invalid body accepted")
	}
	for _, want := range []string{"name", "id"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q should mention %q", err, want)
		}
	}

	if err := ValidateResponse(spec, ref, 200, http.Header{"Content-Type": {"text/html"}}, []byte(`<html>`)); err == nil {
		t.Error("undeclared content type accepted")
	}

	// 404 declares no content: anything goes.
	if err := ValidateResponse(spec, ref, 404, http.Header{}, []byte(`oops`)); err != nil {
		t.Errorf("404 without schema: %v", err)
	}
}

func TestResponseSchemaDeclared(t *testing.T) {
	ref := loadContractSpec(t)
	spec, err := ParseSpec(ref.FilePath)
	if err != nil {
		t.Fatal(err)
	}
	if !ResponseSchemaDeclared(spec, ref, 200) {
		t.Error("200 has a schema")
	}
	if ResponseSchemaDeclared(spec, ref, 404) || ResponseSchemaDeclared(spec, ref, 500) {
		t.Error("404/500 have no schema")
	}
}

const requestSpec = `openapi: 3.0.0
info: {title: t, version: "1"}
paths:
  /users/{id}:
    put:
      parameters:
        - {in: path, name: id, required: true, schema: {type: integer}}
        - {in: query, name: dryRun, schema: {type: boolean}}
        - {in: header, name: X-Request-Id, required: true, schema: {type: string}}
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              required: [name]
              properties: {name: {type: string}, age: {type: integer, minimum: 0}}
      responses:
        "200": {description: OK}
`

func TestValidateRequest(t *testing.T) {
	p := filepath.Join(t.TempDir(), "spec.yml")
	if err := os.WriteFile(p, []byte(requestSpec), 0o644); err != nil {
		t.Fatal(err)
	}
	spec, err := ParseSpec(p)
	if err != nil {
		t.Fatal(err)
	}
	ref := model.OpenAPIRef{FilePath: p, Path: "/users/{id}", Method: "PUT"}
	build := func(query, body string, withHeader bool) *http.Request {
		req, _ := http.NewRequest("PUT", "http://x/users/1"+query, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		if withHeader {
			req.Header.Set("X-Request-Id", "r1")
		}
		return req
	}

	if err := ValidateRequest(spec, ref, build("", `{"name":"ana"}`, true), map[string]string{"id": "1"}); err != nil {
		t.Errorf("valid request rejected: %v", err)
	}

	err = ValidateRequest(spec, ref, build("?dryRun=maybe", `{"age":-1}`, false), map[string]string{"id": "x"})
	if err == nil {
		t.Fatal("invalid request accepted")
	}
	for _, want := range []string{`path parameter "id"`, `query parameter "dryRun"`, `header parameter "X-Request-Id"`, "body", "name"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error should mention %q: %v", want, err)
		}
	}
}
