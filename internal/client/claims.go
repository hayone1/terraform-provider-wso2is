package client

import (
	"context"
	"encoding/base64"
	"net/url"
)

// LocalDialectURI is the URI of the local claim dialect.
const LocalDialectURI = "http://wso2.org/claims"

// ClaimID returns the API identifier of a claim or dialect URI (unpadded base64url).
func ClaimID(uri string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(uri))
}

// DialectID returns the API identifier of a dialect ("local" for the local dialect).
func DialectID(uri string) string {
	if uri == LocalDialectURI {
		return "local"
	}
	return ClaimID(uri)
}

// ---- Local claims (kept as generic maps for read-modify-write updates) ----

func (c *Client) localClaimPath(id string) string {
	return c.ServerPath("/claim-dialects/local/claims/" + url.PathEscape(id))
}

// CreateLocalClaim creates a local claim and returns its ID.
func (c *Client) CreateLocalClaim(ctx context.Context, body map[string]any) (string, error) {
	resp, err := c.Post(ctx, c.ServerPath("/claim-dialects/local/claims"), body, nil)
	if err != nil {
		return "", err
	}
	if id := resp.LocationID(); id != "" {
		return id, nil
	}
	uri, _ := body["claimURI"].(string)
	return ClaimID(uri), nil
}

// GetLocalClaim reads a local claim.
func (c *Client) GetLocalClaim(ctx context.Context, id string) (map[string]any, error) {
	var out map[string]any
	if err := c.Get(ctx, c.localClaimPath(id), nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// PutLocalClaim replaces a local claim.
func (c *Client) PutLocalClaim(ctx context.Context, id string, body map[string]any) error {
	_, err := c.Put(ctx, c.localClaimPath(id), body, nil)
	return err
}

// DeleteLocalClaim deletes a local claim.
func (c *Client) DeleteLocalClaim(ctx context.Context, id string) error {
	return c.Delete(ctx, c.localClaimPath(id))
}

// ListLocalClaims lists every local claim.
func (c *Client) ListLocalClaims(ctx context.Context) ([]map[string]any, error) {
	var out []map[string]any
	if err := c.Get(ctx, c.ServerPath("/claim-dialects/local/claims"), nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ---- Dialects ----

// ClaimDialect is a claim dialect.
type ClaimDialect struct {
	ID         string `json:"id,omitempty"`
	DialectURI string `json:"dialectURI"`
}

// CreateClaimDialect creates an external claim dialect and returns its ID.
func (c *Client) CreateClaimDialect(ctx context.Context, uri string) (string, error) {
	resp, err := c.Post(ctx, c.ServerPath("/claim-dialects"), ClaimDialect{DialectURI: uri}, nil)
	if err != nil {
		return "", err
	}
	if id := resp.LocationID(); id != "" {
		return id, nil
	}
	return ClaimID(uri), nil
}

// GetClaimDialect reads a dialect.
func (c *Client) GetClaimDialect(ctx context.Context, id string) (*ClaimDialect, error) {
	var out ClaimDialect
	if err := c.Get(ctx, c.ServerPath("/claim-dialects/"+url.PathEscape(id)), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// RenameClaimDialect changes a dialect URI; the dialect ID changes with it.
func (c *Client) RenameClaimDialect(ctx context.Context, id, uri string) (string, error) {
	resp, err := c.Put(ctx, c.ServerPath("/claim-dialects/"+url.PathEscape(id)), ClaimDialect{DialectURI: uri}, nil)
	if err != nil {
		return "", err
	}
	if nid := resp.LocationID(); nid != "" {
		return nid, nil
	}
	return ClaimID(uri), nil
}

// DeleteClaimDialect deletes a dialect and its external claims.
func (c *Client) DeleteClaimDialect(ctx context.Context, id string) error {
	return c.Delete(ctx, c.ServerPath("/claim-dialects/"+url.PathEscape(id)))
}

// ListClaimDialects lists every dialect.
func (c *Client) ListClaimDialects(ctx context.Context) ([]ClaimDialect, error) {
	var out []ClaimDialect
	if err := c.Get(ctx, c.ServerPath("/claim-dialects"), nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ---- External claims ----

// ExternalClaim maps a claim of an external dialect to a local claim.
type ExternalClaim struct {
	ID                  string     `json:"id,omitempty"`
	ClaimURI            string     `json:"claimURI"`
	ClaimDialectURI     string     `json:"claimDialectURI,omitempty"`
	MappedLocalClaimURI string     `json:"mappedLocalClaimURI"`
	Properties          []Property `json:"properties,omitempty"`
}

// Property is a key/value pair.
type Property struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

func (c *Client) externalClaimPath(dialectID, id string) string {
	return c.ServerPath("/claim-dialects/" + url.PathEscape(dialectID) + "/claims/" + url.PathEscape(id))
}

// CreateExternalClaim creates an external claim and returns its ID.
func (c *Client) CreateExternalClaim(ctx context.Context, dialectID string, ec *ExternalClaim) (string, error) {
	resp, err := c.Post(ctx, c.ServerPath("/claim-dialects/"+url.PathEscape(dialectID)+"/claims"), ec, nil)
	if err != nil {
		return "", err
	}
	if id := resp.LocationID(); id != "" {
		return id, nil
	}
	return ClaimID(ec.ClaimURI), nil
}

// GetExternalClaim reads an external claim.
func (c *Client) GetExternalClaim(ctx context.Context, dialectID, id string) (*ExternalClaim, error) {
	var out ExternalClaim
	if err := c.Get(ctx, c.externalClaimPath(dialectID, id), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// PutExternalClaim updates an external claim.
func (c *Client) PutExternalClaim(ctx context.Context, dialectID, id string, ec *ExternalClaim) error {
	_, err := c.Put(ctx, c.externalClaimPath(dialectID, id), ec, nil)
	return err
}

// DeleteExternalClaim deletes an external claim.
func (c *Client) DeleteExternalClaim(ctx context.Context, dialectID, id string) error {
	return c.Delete(ctx, c.externalClaimPath(dialectID, id))
}

// ListExternalClaims lists the claims of a dialect.
func (c *Client) ListExternalClaims(ctx context.Context, dialectID string) ([]ExternalClaim, error) {
	var out []ExternalClaim
	if err := c.Get(ctx, c.ServerPath("/claim-dialects/"+url.PathEscape(dialectID)+"/claims"), nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ---- OIDC scopes ----

// OIDCScope groups OIDC claims under a scope name.
type OIDCScope struct {
	Name        string   `json:"name,omitempty"`
	DisplayName string   `json:"displayName"`
	Description string   `json:"description,omitempty"`
	Claims      []string `json:"claims"`
}

// CreateOIDCScope creates an OIDC scope.
func (c *Client) CreateOIDCScope(ctx context.Context, s *OIDCScope) error {
	_, err := c.Post(ctx, c.ServerPath("/oidc/scopes"), s, nil)
	return err
}

// GetOIDCScope reads an OIDC scope.
func (c *Client) GetOIDCScope(ctx context.Context, name string) (*OIDCScope, error) {
	var out OIDCScope
	if err := c.Get(ctx, c.ServerPath("/oidc/scopes/"+url.PathEscape(name)), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// PutOIDCScope updates an OIDC scope (the name cannot change).
func (c *Client) PutOIDCScope(ctx context.Context, name string, s *OIDCScope) error {
	body := map[string]any{"displayName": s.DisplayName, "description": s.Description, "claims": nonNil(s.Claims)}
	_, err := c.Put(ctx, c.ServerPath("/oidc/scopes/"+url.PathEscape(name)), body, nil)
	return err
}

// DeleteOIDCScope deletes an OIDC scope.
func (c *Client) DeleteOIDCScope(ctx context.Context, name string) error {
	return c.Delete(ctx, c.ServerPath("/oidc/scopes/"+url.PathEscape(name)))
}
