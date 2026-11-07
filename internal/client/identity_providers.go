package client

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
)

// AuthenticatorID returns the API ID of an authenticator or connector name.
func AuthenticatorID(name string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(name))
}

// IDPPatchOperation is a JSON-Patch-like operation of the IdP API.
type IDPPatchOperation struct {
	Operation string `json:"operation"`
	Path      string `json:"path"`
	Value     any    `json:"value,omitempty"`
}

func (c *Client) idpPath(id string) string {
	return c.ServerPath("/identity-providers/" + url.PathEscape(id))
}

// CreateIdentityProvider creates an identity provider and returns its ID.
func (c *Client) CreateIdentityProvider(ctx context.Context, body map[string]any) (string, error) {
	resp, err := c.Do(ctx, Request{Method: http.MethodPost, Path: c.ServerPath("/identity-providers"), Body: body})
	if err != nil {
		return "", err
	}
	id := resp.LocationID()
	if id == "" {
		var out map[string]any
		_ = resp.Decode(&out)
		id, _ = out["id"].(string)
	}
	if id == "" {
		return "", fmt.Errorf("identity provider created but the server returned no ID")
	}
	return id, nil
}

// GetIdentityProvider reads an identity provider.
func (c *Client) GetIdentityProvider(ctx context.Context, id string) (map[string]any, error) {
	var out map[string]any
	if err := c.Get(ctx, c.idpPath(id), nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// PatchIdentityProvider applies patch operations to the top-level fields.
func (c *Client) PatchIdentityProvider(ctx context.Context, id string, ops []IDPPatchOperation) error {
	if len(ops) == 0 {
		return nil
	}
	_, err := c.Patch(ctx, c.idpPath(id), ops, nil)
	return err
}

// DeleteIdentityProvider deletes an identity provider. force deletes it even when
// applications still use it.
func (c *Client) DeleteIdentityProvider(ctx context.Context, id string, force bool) error {
	q := url.Values{}
	if force {
		q.Set("force", "true")
	}
	_, err := c.Do(ctx, Request{Method: http.MethodDelete, Path: c.idpPath(id), Query: q})
	return err
}

// GetIdentityProviderSub reads a sub-resource (`federated-authenticators`, `claims`,
// `roles`, `groups`, `provisioning/jit`, `provisioning/outbound-connectors`,
// `implicit-association`).
func (c *Client) GetIdentityProviderSub(ctx context.Context, id, sub string, out any) error {
	return c.Get(ctx, c.idpPath(id)+"/"+sub, nil, out)
}

// PutIdentityProviderSub replaces a sub-resource.
func (c *Client) PutIdentityProviderSub(ctx context.Context, id, sub string, body any) error {
	_, err := c.Put(ctx, c.idpPath(id)+"/"+sub, body, nil)
	return err
}

// IdentityProviderListItem is one entry of GET /identity-providers.
type IdentityProviderListItem struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	IsEnabled   bool   `json:"isEnabled"`
	Image       string `json:"image"`
}

// ListIdentityProviders lists identity providers matching an optional filter
// (for example `name eq Google`), following pagination.
func (c *Client) ListIdentityProviders(ctx context.Context, filter string) ([]IdentityProviderListItem, error) {
	var out []IdentityProviderListItem
	const limit = 100
	for offset := 0; ; offset += limit {
		q := url.Values{"limit": {fmt.Sprint(limit)}, "offset": {fmt.Sprint(offset)}}
		if filter != "" {
			q.Set("filter", filter)
		}
		var page struct {
			TotalResults      int                        `json:"totalResults"`
			IdentityProviders []IdentityProviderListItem `json:"identityProviders"`
		}
		if err := c.Get(ctx, c.ServerPath("/identity-providers"), q, &page); err != nil {
			return nil, err
		}
		out = append(out, page.IdentityProviders...)
		if len(page.IdentityProviders) < limit || len(out) >= page.TotalResults {
			return out, nil
		}
	}
}
