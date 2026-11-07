package client

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// DefaultScopes is requested with the client credentials grant when no scopes are
// configured. WSO2 Identity Server expands SYSTEM to every internal scope the
// application is authorized for.
var DefaultScopes = []string{"SYSTEM"}

type authenticator interface {
	apply(ctx context.Context, req *http.Request) error
	// invalidate drops a cached credential; it returns false when nothing can be refreshed.
	invalidate() bool
}

func newAuthenticator(c *Client, cfg Config) (authenticator, error) {
	hasBasic := cfg.Username != "" || cfg.Password != ""
	hasClient := cfg.ClientID != "" || cfg.ClientSecret != ""
	hasToken := cfg.AccessToken != ""

	n := 0
	for _, b := range []bool{hasBasic, hasClient, hasToken} {
		if b {
			n++
		}
	}
	switch {
	case n == 0:
		return nil, errors.New("no credentials configured: set username and password, client_id and client_secret, or access_token")
	case n > 1:
		return nil, errors.New("conflicting credentials: configure only one of username/password, client_id/client_secret or access_token")
	}

	if hasBasic && (cfg.Username == "" || cfg.Password == "") {
		return nil, errors.New("both username and password are required for basic authentication")
	}
	if hasClient && (cfg.ClientID == "" || cfg.ClientSecret == "") {
		return nil, errors.New("both client_id and client_secret are required for the client credentials grant")
	}

	scopes := cfg.Scopes
	if len(scopes) == 0 {
		scopes = DefaultScopes
	}

	newClientCredentials := func() *clientCredentials {
		return &clientCredentials{c: c, clientID: cfg.ClientID, clientSecret: cfg.ClientSecret, scopes: scopes,
			cache: sharedTokenCache("client_credentials", c.baseURL.String(), c.tenantDomain, "", cfg.ClientID, cfg.ClientSecret, scopes)}
	}

	if cfg.OrganizationID != "" {
		switch {
		case hasToken:
			// The caller supplied a token already issued for the organization.
			return &staticToken{token: cfg.AccessToken}, nil
		case hasClient:
			return &orgSwitch{root: newClientCredentials(), c: c, clientID: cfg.ClientID, clientSecret: cfg.ClientSecret, scopes: scopes, orgID: cfg.OrganizationID,
				cache: sharedTokenCache("organization_switch", c.baseURL.String(), c.tenantDomain, cfg.OrganizationID, cfg.ClientID, cfg.ClientSecret, scopes)}, nil
		default:
			return nil, errors.New("organization_id needs client_id/client_secret of an application allowed to use the organization_switch grant, or an access_token issued for that organization; basic authentication cannot manage sub-organizations")
		}
	}

	switch {
	case hasBasic:
		return &basicAuth{username: cfg.Username, password: cfg.Password}, nil
	case hasClient:
		return newClientCredentials(), nil
	default:
		return &staticToken{token: cfg.AccessToken}, nil
	}
}

type basicAuth struct{ username, password string }

func (a *basicAuth) apply(_ context.Context, r *http.Request) error {
	r.SetBasicAuth(a.username, a.password)
	return nil
}
func (a *basicAuth) invalidate() bool { return false }

type staticToken struct{ token string }

func (a *staticToken) apply(_ context.Context, r *http.Request) error {
	r.Header.Set("Authorization", "Bearer "+a.token)
	return nil
}
func (a *staticToken) invalidate() bool { return false }

var (
	tokenCachesMu sync.Mutex
	tokenCaches   = map[string]*tokenCache{}
)

// sharedTokenCache returns the token cache for one set of credentials. Clients with the
// same server, tenant, organization, client and scopes share the cache, so a new client
// reuses a valid token instead of requesting another one. Crossplane creates a new
// provider, and so a new client, for every reconcile. The key holds a hash of the secret,
// so a changed secret gets a new cache.
func sharedTokenCache(grant, baseURL, tenant, orgID, clientID, clientSecret string, scopes []string) *tokenCache {
	h := sha256.New()
	for _, part := range []string{grant, baseURL, tenant, orgID, clientID, clientSecret, strings.Join(scopes, " ")} {
		h.Write([]byte(part))
		h.Write([]byte{0})
	}
	key := hex.EncodeToString(h.Sum(nil))
	tokenCachesMu.Lock()
	defer tokenCachesMu.Unlock()
	t, ok := tokenCaches[key]
	if !ok {
		t = &tokenCache{}
		tokenCaches[key] = t
	}
	return t
}

// tokenCache holds one bearer token and refreshes it shortly before it expires. Requests
// that need a token while another request fetches one wait for that fetch.
type tokenCache struct {
	mu     sync.Mutex
	token  string
	expiry time.Time
}

func (t *tokenCache) get(ctx context.Context, fetch func(context.Context) (string, time.Duration, error)) (string, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.token != "" && time.Now().Before(t.expiry) {
		return t.token, nil
	}
	tok, ttl, err := fetch(ctx)
	if err != nil {
		return "", err
	}
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	// Refresh a little early so long applies do not race the expiry.
	margin := ttl / 10
	if margin > time.Minute {
		margin = time.Minute
	}
	t.token, t.expiry = tok, time.Now().Add(ttl-margin)
	return tok, nil
}

func (t *tokenCache) reset() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	had := t.token != ""
	t.token = ""
	return had
}

type clientCredentials struct {
	c            *Client
	clientID     string
	clientSecret string
	scopes       []string
	cache        *tokenCache
}

func (a *clientCredentials) token(ctx context.Context) (string, error) {
	return a.cache.get(ctx, func(ctx context.Context) (string, time.Duration, error) {
		form := url.Values{"grant_type": {"client_credentials"}, "scope": {strings.Join(a.scopes, " ")}}
		return a.c.requestToken(ctx, a.clientID, a.clientSecret, "/t/"+url.PathEscape(a.c.tenantDomain)+"/oauth2/token", form)
	})
}

func (a *clientCredentials) apply(ctx context.Context, r *http.Request) error {
	tok, err := a.token(ctx)
	if err != nil {
		return err
	}
	r.Header.Set("Authorization", "Bearer "+tok)
	return nil
}
func (a *clientCredentials) invalidate() bool { return a.cache.reset() }

// orgSwitch exchanges a root organization token for a sub-organization token
// (grant_type=organization_switch).
type orgSwitch struct {
	root         *clientCredentials
	c            *Client
	clientID     string
	clientSecret string
	scopes       []string
	orgID        string
	cache        *tokenCache
}

func (a *orgSwitch) apply(ctx context.Context, r *http.Request) error {
	tok, err := a.cache.get(ctx, func(ctx context.Context) (string, time.Duration, error) {
		rootTok, err := a.root.token(ctx)
		if err != nil {
			return "", 0, err
		}
		form := url.Values{
			"grant_type":             {"organization_switch"},
			"token":                  {rootTok},
			"switching_organization": {a.orgID},
			"scope":                  {strings.Join(a.scopes, " ")},
		}
		return a.c.requestToken(ctx, a.clientID, a.clientSecret, "/t/"+url.PathEscape(a.c.tenantDomain)+"/oauth2/token", form)
	})
	if err != nil {
		return err
	}
	r.Header.Set("Authorization", "Bearer "+tok)
	return nil
}

func (a *orgSwitch) invalidate() bool {
	a.root.invalidate()
	return a.cache.reset()
}

func (c *Client) requestToken(ctx context.Context, clientID, clientSecret, path string, form url.Values) (string, time.Duration, error) {
	u := *c.baseURL
	u.Path = path
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), strings.NewReader(form.Encode()))
	if err != nil {
		return "", 0, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", c.userAgent)
	req.SetBasicAuth(clientID, clientSecret)
	resp, err := c.http.Do(req)
	if err != nil {
		return "", 0, fmt.Errorf("requesting access token: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	var body struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int64  `json:"expires_in"`
		Scope       string `json:"scope"`
		Error       string `json:"error"`
		ErrorDesc   string `json:"error_description"`
	}
	dec := json.NewDecoder(resp.Body)
	_ = dec.Decode(&body)
	if resp.StatusCode != http.StatusOK || body.AccessToken == "" {
		msg := body.ErrorDesc
		if msg == "" {
			msg = body.Error
		}
		if msg == "" {
			msg = resp.Status
		}
		return "", 0, fmt.Errorf("requesting access token from %s (grant %s): %s", path, form.Get("grant_type"), msg)
	}
	if strings.TrimSpace(body.Scope) == "" {
		return "", 0, fmt.Errorf("the access token issued to client %q has no scopes: authorize the application for the management APIs Terraform uses (Console > Applications > API Authorization)", clientID)
	}
	return body.AccessToken, time.Duration(body.ExpiresIn) * time.Second, nil
}
