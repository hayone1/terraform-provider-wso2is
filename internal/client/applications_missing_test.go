package client

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// After an application is deleted, WSO2 IS 7.3.0 keeps answering GET /applications/{id}
// with the old application for about 15 minutes (a cache). The endpoints below the
// application already answer 400 APP-60001 "Application does not belong to the tenant
// domain". Found 2026-11-07 when Crossplane deleted an application and its API
// authorizations at the same time.
func newStaleApplicationServer(t *testing.T, appExists, cached bool) *Client {
	t.Helper()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/authorized-apis"):
			if appExists {
				_, _ = w.Write([]byte(`[]`))
				return
			}
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"code":"APP-60001","message":"Error retrieving authorized APIs of application with id: app-1","description":"Application does not belong to the tenant domain: carbon.super"}`))
		case strings.HasSuffix(r.URL.Path, "/applications/app-1"):
			if appExists || cached {
				_, _ = w.Write([]byte(`{"id":"app-1","name":"x"}`))
				return
			}
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"code":"APP-60006","message":"Application not found"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	c, err := New(Config{BaseURL: srv.URL, Username: "admin", Password: "admin", InsecureSkipVerify: true})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestApplicationMissing(t *testing.T) {
	ctx := context.Background()
	cases := map[string]struct {
		exists, cached, want bool
	}{
		"Exists":                {exists: true, want: false},
		"Deleted":               {exists: false, want: true},
		"DeletedButStillCached": {exists: false, cached: true, want: true},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			c := newStaleApplicationServer(t, tc.exists, tc.cached)
			if got := c.ApplicationMissing(ctx, "app-1"); got != tc.want {
				t.Errorf("ApplicationMissing() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestApplicationGone(t *testing.T) {
	ctx := context.Background()
	notInTenant := &APIError{StatusCode: 400, Code: "APP-60001", Message: "Error while deleting authorized API with id: x from the application with  id: app-1", Description: "Application does not belong to the tenant domain: carbon.super"}
	cases := map[string]struct {
		exists, cached bool
		err            error
		want           bool
	}{
		"NoError":                {exists: true, err: nil, want: false},
		"NotFound":               {exists: true, err: &APIError{StatusCode: 404}, want: true},
		"NotInTenantWhileCached": {exists: false, cached: true, err: notInTenant, want: true},
		"InvalidApplication":     {exists: false, cached: true, err: &APIError{StatusCode: 400, Code: "ORG-60035", Message: "Invalid application"}, want: true},
		"OtherErrorAppExists":    {exists: true, err: &APIError{StatusCode: 500, Code: "SE-50000"}, want: false},
		"OtherErrorAppDeleted":   {exists: false, err: &APIError{StatusCode: 500, Code: "SE-50000"}, want: true},
		"WrappedNotInTenant":     {exists: false, cached: true, err: errors.Join(errors.New("revoke"), notInTenant), want: true},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			c := newStaleApplicationServer(t, tc.exists, tc.cached)
			if got := c.ApplicationGone(ctx, "app-1", tc.err); got != tc.want {
				t.Errorf("ApplicationGone() = %v, want %v", got, tc.want)
			}
		})
	}
}
