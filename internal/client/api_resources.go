package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// APIResource is an API (resource server) with its scopes.
type APIResource struct {
	ID                        string                     `json:"id,omitempty"`
	Name                      string                     `json:"name"`
	Identifier                string                     `json:"identifier"`
	Description               string                     `json:"description,omitempty"`
	Type                      string                     `json:"type,omitempty"`
	RequiresAuthorization     *bool                      `json:"requiresAuthorization,omitempty"`
	Scopes                    []APIScope                 `json:"scopes,omitempty"`
	AuthorizationDetailsTypes []AuthorizationDetailsType `json:"authorizationDetailsTypes,omitempty"`
	Properties                []map[string]any           `json:"properties,omitempty"`
}

// APIScope is a scope (permission) of an API resource.
type APIScope struct {
	ID          string `json:"id,omitempty"`
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
	Description string `json:"description,omitempty"`
}

// AuthorizationDetailsType is a rich authorization request (RFC 9396) type.
type AuthorizationDetailsType struct {
	ID          string          `json:"id,omitempty"`
	Type        string          `json:"type"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Schema      json.RawMessage `json:"schema"`
}

// APIResourceListItem is one entry of GET /api-resources.
type APIResourceListItem struct {
	ID                    string `json:"id"`
	Name                  string `json:"name"`
	Identifier            string `json:"identifier"`
	Type                  string `json:"type"`
	RequiresAuthorization bool   `json:"requiresAuthorization"`
}

func (c *Client) apiResourcePath(id string) string {
	return c.ServerPath("/api-resources/" + url.PathEscape(id))
}

// CreateAPIResource creates an API resource and returns its ID.
func (c *Client) CreateAPIResource(ctx context.Context, r *APIResource) (string, error) {
	var out APIResource
	resp, err := c.Post(ctx, c.ServerPath("/api-resources"), r, &out)
	if err != nil {
		return "", err
	}
	if out.ID != "" {
		return out.ID, nil
	}
	if id := resp.LocationID(); id != "" {
		return id, nil
	}
	return "", fmt.Errorf("API resource created but the server returned no ID")
}

// GetAPIResource reads an API resource.
func (c *Client) GetAPIResource(ctx context.Context, id string) (*APIResource, error) {
	var out APIResource
	if err := c.Get(ctx, c.apiResourcePath(id), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// PatchAPIResource updates the name and description.
func (c *Client) PatchAPIResource(ctx context.Context, id, name, description string) error {
	_, err := c.Patch(ctx, c.apiResourcePath(id), map[string]any{"name": name, "description": description}, nil)
	return err
}

// PutAPIScopes replaces every scope of an API resource.
func (c *Client) PutAPIScopes(ctx context.Context, id string, scopes []APIScope) error {
	_, err := c.Put(ctx, c.apiResourcePath(id)+"/scopes", nonNil(scopes), nil)
	return err
}

// PutAuthorizationDetailsTypes replaces every authorization details type of an API resource.
func (c *Client) PutAuthorizationDetailsTypes(ctx context.Context, id string, types []AuthorizationDetailsType) error {
	_, err := c.Put(ctx, c.apiResourcePath(id)+"/authorization-details-types", nonNil(types), nil)
	return err
}

// DeleteAPIResource deletes an API resource.
func (c *Client) DeleteAPIResource(ctx context.Context, id string) error {
	return c.Delete(ctx, c.apiResourcePath(id))
}

// ListAPIResources lists API resources matching an optional filter (for example
// `identifier eq https://api.example.com` or `type eq BUSINESS`), following cursors.
func (c *Client) ListAPIResources(ctx context.Context, filter string) ([]APIResourceListItem, error) {
	var out []APIResourceListItem
	q := url.Values{"limit": {"100"}}
	if filter != "" {
		q.Set("filter", filter)
	}
	for page := 0; page < 1000; page++ {
		var res struct {
			APIResources []APIResourceListItem `json:"apiResources"`
			Links        []struct {
				Href string `json:"href"`
				Rel  string `json:"rel"`
			} `json:"links"`
		}
		if err := c.Get(ctx, c.ServerPath("/api-resources"), q, &res); err != nil {
			return nil, err
		}
		out = append(out, res.APIResources...)
		next := ""
		for _, l := range res.Links {
			if l.Rel == "next" {
				next = l.Href
			}
		}
		if next == "" || len(res.APIResources) == 0 {
			return out, nil
		}
		// A malformed next link ends the listing like a missing one.
		u, perr := url.Parse(next)
		after := ""
		if perr == nil {
			after = u.Query().Get("after")
		}
		if after == "" {
			return out, nil
		}
		q.Set("after", after)
	}
	return out, nil
}

// FindAPIResourceByIdentifier returns the API resource with the given identifier, or a
// 404 *APIError.
func (c *Client) FindAPIResourceByIdentifier(ctx context.Context, identifier string) (*APIResource, error) {
	items, err := FindByEq(ctx, c.ListAPIResources, "identifier", identifier)
	if err != nil {
		return nil, err
	}
	for _, it := range items {
		if it.Identifier == identifier {
			return c.GetAPIResource(ctx, it.ID)
		}
	}
	// The filter never matches identifiers with parentheses, such as
	// /api/server/v(.*)/notification/(?:email|sms)/template-types: search the full list.
	if strings.ContainsAny(identifier, "()") {
		if items, err = c.ListAPIResources(ctx, ""); err != nil {
			return nil, err
		}
		for _, it := range items {
			if it.Identifier == identifier {
				return c.GetAPIResource(ctx, it.ID)
			}
		}
	}
	return nil, &APIError{Method: http.MethodGet, Path: "/api-resources", StatusCode: http.StatusNotFound, Message: "no API resource with identifier " + identifier}
}
