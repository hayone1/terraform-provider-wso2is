package client

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

// APIError is a non-2xx response from the server.
type APIError struct {
	Method     string
	Path       string
	StatusCode int
	// Code is WSO2's error code, for example APP-60004 or IDP-60002.
	Code        string
	Message     string
	Description string
	TraceID     string
	Body        string
}

func (e *APIError) Error() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s %s: HTTP %d", e.Method, e.Path, e.StatusCode)
	if e.Code != "" {
		fmt.Fprintf(&b, " %s", e.Code)
	}
	switch {
	case e.Message != "" && e.Description != "" && e.Description != e.Message:
		fmt.Fprintf(&b, ": %s: %s", e.Message, e.Description)
	case e.Message != "":
		fmt.Fprintf(&b, ": %s", e.Message)
	case e.Description != "":
		fmt.Fprintf(&b, ": %s", e.Description)
	case e.Body != "":
		fmt.Fprintf(&b, ": %s", truncate(e.Body, 1024))
	}
	if e.TraceID != "" {
		fmt.Fprintf(&b, " (trace ID %s)", e.TraceID)
	}
	return b.String()
}

func newAPIError(method, path string, resp *Response) *APIError {
	e := &APIError{Method: method, Path: path, StatusCode: resp.StatusCode, Body: strings.TrimSpace(string(resp.Body))}
	var body struct {
		// Server APIs.
		Code        string `json:"code"`
		Message     string `json:"message"`
		Description string `json:"description"`
		TraceID     string `json:"traceId"`
		RefID       string `json:"ref"`
		// SCIM.
		Detail   string `json:"detail"`
		ScimType string `json:"scimType"`
		// OAuth2 token endpoint.
		OAuthError     string `json:"error"`
		OAuthErrorDesc string `json:"error_description"`
	}
	if json.Unmarshal(resp.Body, &body) == nil {
		e.Code = body.Code
		e.Message = body.Message
		e.Description = body.Description
		e.TraceID = body.TraceID
		if e.TraceID == "" {
			e.TraceID = body.RefID
		}
		if body.Detail != "" {
			e.Message = body.Detail
			if body.ScimType != "" {
				e.Code = body.ScimType
			}
		}
		if body.OAuthError != "" {
			e.Code = body.OAuthError
			e.Message = body.OAuthErrorDesc
		}
	}
	return e
}

// StatusCode returns the HTTP status of an *APIError, or 0.
func StatusCode(err error) int {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.StatusCode
	}
	return 0
}

// IsNotFound reports whether err means the object does not exist.
func IsNotFound(err error) bool {
	return StatusCode(err) == http.StatusNotFound
}

// IsConflict reports whether err is an HTTP 409.
func IsConflict(err error) bool {
	return StatusCode(err) == http.StatusConflict
}

// HasCode reports whether err is an *APIError with one of the given WSO2 error codes.
func HasCode(err error, codes ...string) bool {
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		return false
	}
	for _, c := range codes {
		if apiErr.Code == c {
			return true
		}
	}
	return false
}

func errorAs(err error, target **APIError) bool { return errors.As(err, target) }
