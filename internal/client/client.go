// Package client is a small HTTP client for the WSO2 Identity Server management REST APIs.
//
// It handles authentication (basic, OAuth2 client credentials, static bearer tokens and
// organization-switched tokens), tenant and organization scoped paths, retries for
// transient failures and WSO2's error formats. API specific methods live in the other
// files of this package.
package client

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// DefaultTenantDomain is the tenant every WSO2 Identity Server starts with.
const DefaultTenantDomain = "carbon.super"

// Config holds everything needed to build a Client.
type Config struct {
	// BaseURL is the server origin, for example https://localhost:9443.
	BaseURL string
	// TenantDomain selects the tenant (root organization). Defaults to carbon.super.
	TenantDomain string
	// OrganizationID, when set, scopes every request to that sub-organization. It needs a
	// token for the organization: an OAuth2 client (ClientID/ClientSecret) able to use the
	// organization_switch grant, or an AccessToken already issued for the organization.
	OrganizationID string

	Username string
	Password string

	ClientID     string
	ClientSecret string
	Scopes       []string

	AccessToken string

	InsecureSkipVerify bool
	CACertPEM          string

	Timeout    time.Duration
	MaxRetries int
	UserAgent  string
}

// Client talks to one tenant (and optionally one organization) of a WSO2 Identity Server.
type Client struct {
	baseURL      *url.URL
	tenantDomain string
	orgID        string
	http         *http.Client
	auth         authenticator
	maxRetries   int
	userAgent    string
	// unavailable is returned by every request of a client whose configuration is not
	// known yet (see NewUnavailable).
	unavailable error
}

// NewUnavailable returns a client whose requests fail with reason. The provider uses it
// while its configuration depends on values known only during apply: planning new
// resources needs no API calls, and Terraform configures the provider again before
// applying them.
func NewUnavailable(reason string) *Client {
	return &Client{baseURL: &url.URL{Scheme: "https", Host: "unavailable.invalid"}, unavailable: errors.New(reason)}
}

// New validates the configuration and returns a ready client. It does not contact the server.
func New(cfg Config) (*Client, error) {
	if cfg.BaseURL == "" {
		return nil, errors.New("base URL is required")
	}
	u, err := url.Parse(strings.TrimRight(cfg.BaseURL, "/"))
	if err != nil || u.Scheme == "" || u.Host == "" {
		return nil, fmt.Errorf("invalid base URL %q: must be an absolute URL such as https://localhost:9443", cfg.BaseURL)
	}
	if u.Path != "" && u.Path != "/" {
		return nil, fmt.Errorf("invalid base URL %q: use the server origin without a path; set the tenant with tenant_domain", cfg.BaseURL)
	}
	u.Path = ""

	tlsCfg := &tls.Config{MinVersion: tls.VersionTLS12, InsecureSkipVerify: cfg.InsecureSkipVerify} //nolint:gosec // opt-in for lab servers
	if cfg.CACertPEM != "" {
		pool, err := x509.SystemCertPool()
		if err != nil || pool == nil {
			pool = x509.NewCertPool()
		}
		if !pool.AppendCertsFromPEM([]byte(cfg.CACertPEM)) {
			return nil, errors.New("ca certificate: no PEM certificates found")
		}
		tlsCfg.RootCAs = pool
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = tlsCfg

	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	tenant := cfg.TenantDomain
	if tenant == "" {
		tenant = DefaultTenantDomain
	}
	c := &Client{
		baseURL:      u,
		tenantDomain: tenant,
		orgID:        cfg.OrganizationID,
		http:         &http.Client{Timeout: timeout, Transport: transport},
		maxRetries:   cfg.MaxRetries,
		userAgent:    cfg.UserAgent,
	}
	if c.maxRetries < 0 {
		c.maxRetries = 0
	}
	if c.userAgent == "" {
		c.userAgent = "terraform-provider-wso2is"
	}

	c.auth, err = newAuthenticator(c, cfg)
	if err != nil {
		return nil, err
	}
	return c, nil
}

// WithBasicAuth returns a client for another tenant that shares this client's connection
// settings (TLS, timeouts, retries) and authenticates with username and password.
func (c *Client) WithBasicAuth(tenantDomain, username, password string) *Client {
	return &Client{
		baseURL:      c.baseURL,
		tenantDomain: tenantDomain,
		http:         c.http,
		auth:         &basicAuth{username: username, password: password},
		maxRetries:   c.maxRetries,
		userAgent:    c.userAgent,
		unavailable:  c.unavailable,
	}
}

var (
	tenantLocksMu sync.Mutex
	tenantLocks   = map[string]*sync.Mutex{}
)

// LockTenantConfig serializes updates of tenant-wide settings that the server stores in
// one record (governance connectors and server configurations live in the resident
// identity provider): concurrent updates overwrite each other. The lock is shared by all
// clients of the same server and tenant in this process. It returns the unlock function.
func (c *Client) LockTenantConfig() func() {
	key := c.baseURL.String() + c.tenantPrefix()
	tenantLocksMu.Lock()
	mu, ok := tenantLocks[key]
	if !ok {
		mu = &sync.Mutex{}
		tenantLocks[key] = mu
	}
	tenantLocksMu.Unlock()
	mu.Lock()
	return mu.Unlock
}

// TenantDomain returns the tenant this client manages.
func (c *Client) TenantDomain() string { return c.tenantDomain }

// OrganizationID returns the sub-organization this client manages, or "" for the root.
func (c *Client) OrganizationID() string { return c.orgID }

// BaseURL returns the server origin.
func (c *Client) BaseURL() string { return c.baseURL.String() }

// tenantPrefix is /t/{tenant} plus /o when the client is scoped to a sub-organization.
func (c *Client) tenantPrefix() string {
	p := "/t/" + url.PathEscape(c.tenantDomain)
	if c.orgID != "" {
		p += "/o"
	}
	return p
}

// ServerPath returns the path of a server API v1 endpoint, e.g. /t/carbon.super/api/server/v1/applications.
func (c *Client) ServerPath(p string) string { return c.tenantPrefix() + "/api/server/v1" + p }

// ServerV2Path returns the path of a server API v2 endpoint.
func (c *Client) ServerV2Path(p string) string { return c.tenantPrefix() + "/api/server/v2" + p }

// SCIMPath returns the path of a SCIM 2.0 endpoint, e.g. /t/carbon.super/scim2/Users.
func (c *Client) SCIMPath(p string) string { return c.tenantPrefix() + "/scim2" + p }

// TenantPath returns a path under the tenant (and organization) prefix.
func (c *Client) TenantPath(p string) string { return c.tenantPrefix() + p }

// RootPath returns a path that is not tenant qualified (for example the tenant management API).
func (c *Client) RootPath(p string) string { return p }

// Request describes one API call.
type Request struct {
	Method string
	Path   string
	Query  url.Values
	// Body is JSON encoded unless it is an io.Reader or RawBody is set.
	Body        any
	ContentType string
	Accept      string
	// NoAuth skips the Authorization header (token endpoint, health checks).
	NoAuth bool
}

// Response is a completed API call.
type Response struct {
	StatusCode int
	Header     http.Header
	Body       []byte
}

// LocationID returns the last path segment of the Location header, which is how most
// WSO2 create endpoints report the identifier of the new object.
func (r *Response) LocationID() string {
	loc := r.Header.Get("Location")
	if loc == "" {
		return ""
	}
	if u, err := url.Parse(loc); err == nil {
		loc = u.Path
	}
	loc = strings.TrimRight(loc, "/")
	if i := strings.LastIndex(loc, "/"); i >= 0 {
		loc = loc[i+1:]
	}
	if s, err := url.PathUnescape(loc); err == nil {
		return s
	}
	return loc
}

// Decode unmarshals the response body into out.
func (r *Response) Decode(out any) error {
	if out == nil || len(bytes.TrimSpace(r.Body)) == 0 {
		return nil
	}
	if err := json.Unmarshal(r.Body, out); err != nil {
		return fmt.Errorf("decoding response: %w (body: %s)", err, truncate(string(r.Body), 512))
	}
	return nil
}

// Do sends the request, retrying transient failures, and returns an *APIError for any
// non-2xx status.
func (c *Client) Do(ctx context.Context, req Request) (*Response, error) {
	if c.unavailable != nil {
		return nil, c.unavailable
	}
	var body []byte
	switch b := req.Body.(type) {
	case nil:
	case []byte:
		body = b
	case io.Reader:
		var err error
		if body, err = io.ReadAll(b); err != nil {
			return nil, err
		}
	default:
		var err error
		if body, err = json.Marshal(b); err != nil {
			return nil, fmt.Errorf("encoding request body: %w", err)
		}
		if req.ContentType == "" {
			req.ContentType = "application/json"
		}
	}

	retriedAuth := false
	for attempt := 0; ; attempt++ {
		resp, err := c.send(ctx, req, body)
		if err == nil && resp.StatusCode == http.StatusUnauthorized && !req.NoAuth && !retriedAuth && c.auth.invalidate() {
			// Cached token expired or was revoked: fetch a new one once.
			retriedAuth = true
			attempt--
			continue
		}
		if attempt < c.maxRetries && c.retryable(req.Method, resp, err) {
			wait := backoff(attempt, resp)
			tflog.Debug(ctx, "retrying WSO2 request", map[string]any{"method": req.Method, "path": req.Path, "attempt": attempt + 1, "wait": wait.String(), "error": errString(err)})
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(wait):
			}
			continue
		}
		if err != nil {
			return nil, err
		}
		if resp.StatusCode < 200 || resp.StatusCode > 299 {
			return resp, newAPIError(req.Method, req.Path, resp)
		}
		return resp, nil
	}
}

func (c *Client) send(ctx context.Context, req Request, body []byte) (*Response, error) {
	u := *c.baseURL
	u.Path = req.Path
	if strings.ContainsAny(req.Path, "%") {
		// Path already contains escaped segments; keep them as given.
		u.RawPath = req.Path
		if p, err := url.PathUnescape(req.Path); err == nil {
			u.Path = p
		}
	}
	if len(req.Query) > 0 {
		u.RawQuery = req.Query.Encode()
	}

	var rdr io.Reader
	if body != nil {
		rdr = bytes.NewReader(body)
	}
	hreq, err := http.NewRequestWithContext(ctx, req.Method, u.String(), rdr)
	if err != nil {
		return nil, err
	}
	if req.ContentType != "" {
		hreq.Header.Set("Content-Type", req.ContentType)
	}
	accept := req.Accept
	if accept == "" {
		accept = "application/json"
	}
	hreq.Header.Set("Accept", accept)
	hreq.Header.Set("User-Agent", c.userAgent)
	if !req.NoAuth {
		if err := c.auth.apply(ctx, hreq); err != nil {
			return nil, err
		}
	}

	start := time.Now()
	hresp, err := c.http.Do(hreq)
	if err != nil {
		tflog.Debug(ctx, "WSO2 request failed", map[string]any{"method": req.Method, "url": u.String(), "error": err.Error()})
		return nil, err
	}
	defer func() { _ = hresp.Body.Close() }()
	respBody, err := io.ReadAll(io.LimitReader(hresp.Body, 32<<20))
	if err != nil {
		return nil, err
	}
	fields := map[string]any{
		"method":   req.Method,
		"url":      u.String(),
		"status":   hresp.StatusCode,
		"duration": time.Since(start).String(),
	}
	tflog.Debug(ctx, "WSO2 request", fields)
	if body != nil {
		fields["request_body"] = Redact(string(body))
	}
	fields["response_body"] = Redact(truncate(string(respBody), 16<<10))
	tflog.Trace(ctx, "WSO2 request payloads", fields)
	return &Response{StatusCode: hresp.StatusCode, Header: hresp.Header, Body: respBody}, nil
}

func (c *Client) retryable(method string, resp *Response, err error) bool {
	if err != nil {
		var netErr net.Error
		if errors.As(err, &netErr) || errors.Is(err, io.ErrUnexpectedEOF) || strings.Contains(err.Error(), "connection reset") || strings.Contains(err.Error(), "connection refused") {
			// Only repeat requests that are safe to send twice.
			return method != http.MethodPost && method != http.MethodPatch || strings.Contains(err.Error(), "connection refused")
		}
		return false
	}
	switch resp.StatusCode {
	case http.StatusTooManyRequests, http.StatusServiceUnavailable:
		return true
	case http.StatusBadGateway, http.StatusGatewayTimeout:
		return method == http.MethodGet || method == http.MethodPut || method == http.MethodDelete
	}
	return false
}

func backoff(attempt int, resp *Response) time.Duration {
	if resp != nil {
		if s := resp.Header.Get("Retry-After"); s != "" {
			if secs, err := strconv.Atoi(s); err == nil && secs > 0 && secs < 120 {
				return time.Duration(secs) * time.Second
			}
		}
	}
	// 500ms doubling per attempt, capped at 15s. Capping the exponent first keeps the
	// shift from overflowing when max_retries is large.
	d := 15 * time.Second
	if attempt < 5 {
		d = time.Duration(500*(1<<attempt)) * time.Millisecond
	}
	if d > 15*time.Second {
		d = 15 * time.Second
	}
	return d + time.Duration(rand.Int64N(int64(250*time.Millisecond)))
}

// Get is a convenience wrapper that decodes a JSON response into out.
func (c *Client) Get(ctx context.Context, path string, query url.Values, out any) error {
	resp, err := c.Do(ctx, Request{Method: http.MethodGet, Path: path, Query: query})
	if err != nil {
		return err
	}
	return resp.Decode(out)
}

// Post sends body as JSON and decodes the response into out (if non-nil).
func (c *Client) Post(ctx context.Context, path string, body, out any) (*Response, error) {
	resp, err := c.Do(ctx, Request{Method: http.MethodPost, Path: path, Body: body})
	if err != nil {
		return resp, err
	}
	return resp, resp.Decode(out)
}

// Put sends body as JSON and decodes the response into out (if non-nil).
func (c *Client) Put(ctx context.Context, path string, body, out any) (*Response, error) {
	resp, err := c.Do(ctx, Request{Method: http.MethodPut, Path: path, Body: body})
	if err != nil {
		return resp, err
	}
	return resp, resp.Decode(out)
}

// Patch sends body as JSON and decodes the response into out (if non-nil).
func (c *Client) Patch(ctx context.Context, path string, body, out any) (*Response, error) {
	resp, err := c.Do(ctx, Request{Method: http.MethodPatch, Path: path, Body: body})
	if err != nil {
		return resp, err
	}
	return resp, resp.Decode(out)
}

// Delete deletes the object at path. A 404 is returned as an error; callers usually
// treat it as success with IsNotFound.
func (c *Client) Delete(ctx context.Context, path string) error {
	_, err := c.Do(ctx, Request{Method: http.MethodDelete, Path: path})
	return err
}

var secretJSON = regexp.MustCompile(`(?i)("(?:[a-z_]*password|[a-z_]*secret|clientsecret|client_secret|access_token|refresh_token|id_token|token|value|apikey|api_key|privatekey)"\s*:\s*)"(?:[^"\\]|\\.)*"`)

// Redact masks values of JSON keys that commonly hold secrets before they are logged.
func Redact(s string) string {
	return secretJSON.ReplaceAllString(s, `$1"***"`)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "...(truncated)"
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// PathEscape escapes a single path segment.
func PathEscape(s string) string { return url.PathEscape(s) }
