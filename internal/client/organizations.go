package client

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
)

// Organization is a (sub-)organization.
type Organization struct {
	ID           string `json:"id,omitempty"`
	Name         string `json:"name"`
	OrgHandle    string `json:"orgHandle,omitempty"`
	Description  string `json:"description,omitempty"`
	Status       string `json:"status,omitempty"`
	Type         string `json:"type,omitempty"`
	ParentID     string `json:"parentId,omitempty"`
	Version      string `json:"version,omitempty"`
	Created      string `json:"created,omitempty"`
	LastModified string `json:"lastModified,omitempty"`
	HasChildren  bool   `json:"hasChildren,omitempty"`
	Parent       *struct {
		ID string `json:"id"`
	} `json:"parent,omitempty"`
	Attributes []Attribute `json:"attributes,omitempty"`
}

// Attribute is a key/value pair of an organization.
type Attribute struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

func (c *Client) orgPath(id string) string {
	return c.ServerPath("/organizations/" + url.PathEscape(id))
}

// orgNotFound maps the codes returned for organizations that do not exist (or no longer
// exist in the caller's hierarchy) to a 404.
func orgNotFound(err error) error {
	var apiErr *APIError
	if errorAs(err, &apiErr) && (apiErr.Code == "ORG-60064" || apiErr.Code == "ORG-60015" || apiErr.Code == "ORG-60006") {
		e := *apiErr
		e.StatusCode = http.StatusNotFound
		return &e
	}
	return err
}

// CreateOrganization creates an organization and returns its ID.
func (c *Client) CreateOrganization(ctx context.Context, o *Organization) (string, error) {
	var out Organization
	resp, err := c.Post(ctx, c.ServerPath("/organizations"), o, &out)
	if err != nil {
		return "", err
	}
	if out.ID != "" {
		return out.ID, nil
	}
	if id := resp.LocationID(); id != "" {
		return id, nil
	}
	return "", fmt.Errorf("organization created but the server returned no ID")
}

// GetOrganization reads an organization.
func (c *Client) GetOrganization(ctx context.Context, id string) (*Organization, error) {
	var out Organization
	if err := c.Get(ctx, c.orgPath(id), nil, &out); err != nil {
		return nil, orgNotFound(err)
	}
	return &out, nil
}

// PutOrganization replaces an organization's name, description, status and attributes.
func (c *Client) PutOrganization(ctx context.Context, id string, body map[string]any) error {
	_, err := c.Put(ctx, c.orgPath(id), body, nil)
	return orgNotFound(err)
}

// DeleteOrganization deletes an organization (it must have no child organizations).
func (c *Client) DeleteOrganization(ctx context.Context, id string) error {
	return orgNotFound(c.Delete(ctx, c.orgPath(id)))
}

// ListOrganizations lists organizations matching an optional filter (for example
// `name eq Acme`) under the current organization; recursive includes descendants.
func (c *Client) ListOrganizations(ctx context.Context, filter string, recursive bool) ([]Organization, error) {
	var out []Organization
	q := url.Values{"limit": {"100"}}
	if filter != "" {
		q.Set("filter", filter)
	}
	if recursive {
		q.Set("recursive", "true")
	}
	for page := 0; page < 1000; page++ {
		var res struct {
			Organizations []Organization `json:"organizations"`
			Links         []struct {
				Href string `json:"href"`
				Rel  string `json:"rel"`
			} `json:"links"`
		}
		if err := c.Get(ctx, c.ServerPath("/organizations"), q, &res); err != nil {
			return nil, err
		}
		out = append(out, res.Organizations...)
		next := ""
		for _, l := range res.Links {
			if l.Rel == "next" {
				if u, err := url.Parse(l.Href); err == nil {
					next = u.Query().Get("after")
				}
			}
		}
		if next == "" {
			return out, nil
		}
		q.Set("after", next)
	}
	return out, nil
}
