package store

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	"github.com/githiago-f/lazyapi/internal/model"
)

// ResponseSchemaDeclared reports whether the operation declares a body
// schema for this status (exact code, NXX range, or default).
func ResponseSchemaDeclared(spec *openapi3.T, ref model.OpenAPIRef, status int) bool {
	resp := declaredResponse(spec, ref, status)
	if resp == nil {
		return false
	}
	for _, mt := range resp.Content {
		if mt != nil && mt.Schema != nil {
			return true
		}
	}
	return false
}

func declaredResponse(spec *openapi3.T, ref model.OpenAPIRef, status int) *openapi3.Response {
	op := FindOperation(spec, ref)
	if op == nil || op.Responses == nil {
		return nil
	}
	code := strconv.Itoa(status)
	for _, key := range []string{code, code[:1] + "XX", code[:1] + "xx", "default"} {
		if r := op.Responses.Value(key); r != nil && r.Value != nil {
			return r.Value
		}
	}
	return nil
}

// ValidateResponse checks a response's headers and body against the
// operation's declared response for that status. Undeclared statuses are not
// reported here (see StatusDeclared).
func ValidateResponse(spec *openapi3.T, ref model.OpenAPIRef, status int, header http.Header, body []byte) error {
	method := strings.ToUpper(ref.Method)
	op := FindOperation(spec, ref)
	if op == nil {
		return fmt.Errorf("operation %s %s not found in spec", method, ref.Path)
	}
	httpReq, err := http.NewRequest(method, "http://lazyapi.invalid"+ref.Path, nil)
	if err != nil {
		return err
	}
	route := &routers.Route{
		Spec:      spec,
		Path:      ref.Path,
		PathItem:  spec.Paths.Find(ref.Path),
		Method:    method,
		Operation: op,
	}
	input := &openapi3filter.ResponseValidationInput{
		RequestValidationInput: &openapi3filter.RequestValidationInput{
			Request: httpReq,
			Route:   route,
			Options: &openapi3filter.Options{
				ExcludeRequestBody:    true,
				AuthenticationFunc:    openapi3filter.NoopAuthenticationFunc,
				IncludeResponseStatus: false,
			},
		},
		Status: status,
		Header: header,
		Options: &openapi3filter.Options{
			IncludeResponseStatus: false,
			MultiError:            true,
		},
	}
	input.SetBodyBytes(body)
	if err := openapi3filter.ValidateResponse(context.Background(), input); err != nil {
		return contractError(err)
	}
	return nil
}

// ValidateRequest checks an outgoing (or, for the mock server, incoming)
// request against the operation: required parameters, parameter schemas and
// the request body. Security requirements are not enforced. pathParams holds
// the raw path parameter values.
func ValidateRequest(spec *openapi3.T, ref model.OpenAPIRef, req *http.Request, pathParams map[string]string) error {
	method := strings.ToUpper(ref.Method)
	op := FindOperation(spec, ref)
	if op == nil {
		return fmt.Errorf("operation %s %s not found in spec", method, ref.Path)
	}
	input := &openapi3filter.RequestValidationInput{
		Request:    req,
		PathParams: pathParams,
		Route: &routers.Route{
			Spec:      spec,
			Path:      ref.Path,
			PathItem:  spec.Paths.Find(ref.Path),
			Method:    method,
			Operation: op,
		},
		Options: &openapi3filter.Options{
			AuthenticationFunc: openapi3filter.NoopAuthenticationFunc,
			MultiError:         true,
		},
	}
	err := openapi3filter.ValidateRequest(context.Background(), input)
	if err == nil {
		return nil
	}
	// Body schema errors come back bare; parameter errors are wrapped.
	var multi openapi3.MultiError
	if !errors.As(err, &multi) {
		multi = openapi3.MultiError{err}
	}
	parts := make([]string, 0, len(multi))
	for _, e := range multi {
		var schemaErr *openapi3.SchemaError
		var reqErr *openapi3filter.RequestError
		if errors.As(e, &schemaErr) && !errors.As(e, &reqErr) {
			parts = append(parts, "body: "+contractError(e).Error())
			continue
		}
		parts = append(parts, contractError(e).Error())
	}
	return errors.New(strings.Join(parts, "; "))
}

// contractError flattens openapi3filter errors into one readable line per
// problem, dropping the repeated schema dumps.
func contractError(err error) error {
	// Wrappers that carry context (which parameter, the body) are handled
	// before errors.As would unwrap them into a bare MultiError.
	switch err.(type) {
	case *openapi3filter.RequestError, *openapi3filter.ResponseError:
		return wrapperError(err)
	}
	var multi openapi3.MultiError
	if errors.As(err, &multi) {
		parts := make([]string, 0, len(multi))
		for _, e := range multi {
			parts = append(parts, contractError(e).Error())
		}
		return errors.New(strings.Join(parts, "; "))
	}
	var schemaErr *openapi3.SchemaError
	if errors.As(err, &schemaErr) {
		path := strings.Join(schemaErr.JSONPointer(), ".")
		if path == "" {
			return errors.New(schemaErr.Reason)
		}
		return fmt.Errorf("%s: %s", path, schemaErr.Reason)
	}
	return err
}

func wrapperError(err error) error {
	var reqErr *openapi3filter.RequestError
	if errors.As(err, &reqErr) {
		where := "request"
		switch {
		case reqErr.Parameter != nil:
			where = fmt.Sprintf("%s parameter %q", reqErr.Parameter.In, reqErr.Parameter.Name)
		case reqErr.RequestBody != nil:
			where = "body"
		}
		msg := reqErr.Reason
		if reqErr.Err != nil {
			inner := contractError(reqErr.Err).Error()
			switch {
			case msg == "" || strings.Contains(inner, msg):
				msg = inner
			case !strings.Contains(msg, inner):
				msg += ": " + inner
			}
		}
		return fmt.Errorf("%s: %s", where, msg)
	}
	var respErr *openapi3filter.ResponseError
	if errors.As(err, &respErr) {
		if respErr.Err != nil {
			return contractError(respErr.Err)
		}
		return errors.New(respErr.Reason)
	}
	return err
}
