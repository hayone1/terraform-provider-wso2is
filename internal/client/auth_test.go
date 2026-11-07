package client

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

// fakeServer answers the token endpoint and one API endpoint. The API accepts only the
// newest token, so a test can make it reject the cached one.
type fakeServer struct {
	*httptest.Server
	tokenRequests atomic.Int32
	mu            sync.Mutex
	current       string
}

func newFakeServer(t *testing.T) *fakeServer {
	t.Helper()
	f := &fakeServer{}
	f.Server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/oauth2/token"):
			n := f.tokenRequests.Add(1)
			f.mu.Lock()
			f.current = fmt.Sprintf("token-%d", n)
			tok := f.current
			f.mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprintf(w, `{"access_token":%q,"expires_in":3600,"scope":"SYSTEM"}`, tok)
		default:
			f.mu.Lock()
			ok := r.Header.Get("Authorization") == "Bearer "+f.current
			f.mu.Unlock()
			if !ok {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			_, _ = w.Write([]byte(`{}`))
		}
	}))
	t.Cleanup(f.Close)
	return f
}

// revoke makes the server reject every token it issued so far.
func (f *fakeServer) revoke() {
	f.mu.Lock()
	f.current = "revoked"
	f.mu.Unlock()
}

func (f *fakeServer) client(t *testing.T, cfg Config) *Client {
	t.Helper()
	cfg.BaseURL = f.URL
	cfg.InsecureSkipVerify = true
	c, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func get(t *testing.T, c *Client) {
	t.Helper()
	var out any
	if err := c.Get(context.Background(), c.ServerPath("/applications"), nil, &out); err != nil {
		t.Fatal(err)
	}
}

// Crossplane creates a new provider, and so a new client, for every reconcile. The clients
// must share the token instead of requesting a new one each time.
func TestClientCredentialsTokenIsSharedBetweenClients(t *testing.T) {
	f := newFakeServer(t)
	cfg := Config{TenantDomain: "shared.example", ClientID: "id", ClientSecret: "secret"}

	for range 3 {
		get(t, f.client(t, cfg))
	}
	if n := f.tokenRequests.Load(); n != 1 {
		t.Fatalf("three clients with the same credentials requested %d tokens, want 1", n)
	}

	other := cfg
	other.ClientSecret = "rotated"
	get(t, f.client(t, other))
	if n := f.tokenRequests.Load(); n != 2 {
		t.Fatalf("a changed client secret must not reuse the cached token: %d token requests, want 2", n)
	}

	otherScopes := cfg
	otherScopes.Scopes = []string{"internal_user_mgt_view"}
	get(t, f.client(t, otherScopes))
	if n := f.tokenRequests.Load(); n != 3 {
		t.Fatalf("different scopes must not reuse the cached token: %d token requests, want 3", n)
	}
}

// When the server rejects the shared token, one client fetches a new token, and the other
// clients use the new one.
func TestSharedTokenIsReplacedAfterRejection(t *testing.T) {
	f := newFakeServer(t)
	cfg := Config{TenantDomain: "rejected.example", ClientID: "id", ClientSecret: "secret"}
	first, second := f.client(t, cfg), f.client(t, cfg)

	get(t, first)
	f.revoke()
	get(t, first)
	get(t, second)
	if n := f.tokenRequests.Load(); n != 2 {
		t.Fatalf("got %d token requests, want 2 (the first token and one replacement)", n)
	}
}

// Many clients that start at the same time wait for one token request.
func TestConcurrentClientsRequestOneToken(t *testing.T) {
	f := newFakeServer(t)
	cfg := Config{TenantDomain: "concurrent.example", ClientID: "id", ClientSecret: "secret"}
	var wg sync.WaitGroup
	for range 10 {
		c := f.client(t, cfg)
		wg.Go(func() { get(t, c) })
	}
	wg.Wait()
	if n := f.tokenRequests.Load(); n != 1 {
		t.Fatalf("got %d token requests, want 1", n)
	}
}

func TestClientsShareTransportsByTLSSettings(t *testing.T) {
	a, err := New(Config{BaseURL: "https://a.example", Username: "u", Password: "p"})
	if err != nil {
		t.Fatal(err)
	}
	b, err := New(Config{BaseURL: "https://b.example", Username: "u", Password: "p"})
	if err != nil {
		t.Fatal(err)
	}
	insecure, err := New(Config{BaseURL: "https://a.example", Username: "u", Password: "p", InsecureSkipVerify: true})
	if err != nil {
		t.Fatal(err)
	}
	if a.http.Transport != b.http.Transport {
		t.Error("clients with the same TLS settings should share a transport")
	}
	if a.http.Transport == insecure.http.Transport {
		t.Error("clients with different TLS settings must not share a transport")
	}
	if _, err := New(Config{BaseURL: "https://a.example", Username: "u", Password: "p", CACertPEM: "not a certificate"}); err == nil {
		t.Error("an invalid CA certificate should be rejected")
	}
}
