package client

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
)

// GovernanceProperty is a governance connector property.
type GovernanceProperty struct {
	Name        string `json:"name"`
	Value       string `json:"value"`
	DisplayName string `json:"displayName,omitempty"`
	Description string `json:"description,omitempty"`
	Meta        *struct {
		Type    string `json:"type"`
		Regex   string `json:"regex"`
		GroupID int    `json:"groupId"`
	} `json:"meta,omitempty"`
}

// GovernanceConnector is a governance connector (for example account.lock.handler).
type GovernanceConnector struct {
	ID           string               `json:"id"`
	Name         string               `json:"name"`
	Category     string               `json:"category"`
	FriendlyName string               `json:"friendlyName"`
	SubCategory  string               `json:"subCategory"`
	Order        int                  `json:"order"`
	Properties   []GovernanceProperty `json:"properties"`
}

// GovernanceCategory groups connectors.
type GovernanceCategory struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Connectors []struct {
		ID string `json:"id"`
	} `json:"connectors"`
}

// ListGovernanceCategories lists categories with their connector IDs.
func (c *Client) ListGovernanceCategories(ctx context.Context) ([]GovernanceCategory, error) {
	var out []GovernanceCategory
	if err := c.Get(ctx, c.ServerPath("/identity-governance"), nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// FindGovernanceConnector resolves a connector by name or ID and returns its category ID.
func (c *Client) FindGovernanceConnector(ctx context.Context, nameOrID string) (categoryID, connectorID string, err error) {
	cats, err := c.ListGovernanceCategories(ctx)
	if err != nil {
		return "", "", err
	}
	byName := AuthenticatorID(nameOrID)
	for _, cat := range cats {
		for _, con := range cat.Connectors {
			if con.ID == nameOrID || con.ID == byName {
				return cat.ID, con.ID, nil
			}
		}
	}
	return "", "", &APIError{Method: http.MethodGet, Path: "/identity-governance", StatusCode: http.StatusNotFound, Message: fmt.Sprintf("no governance connector named %q", nameOrID)}
}

func (c *Client) governancePath(categoryID, connectorID string) string {
	return c.ServerPath("/identity-governance/" + url.PathEscape(categoryID) + "/connectors/" + url.PathEscape(connectorID))
}

// GetGovernanceConnector reads a connector. Reads are serialized with updates: the
// properties live in the resident identity provider, which updates rewrite.
func (c *Client) GetGovernanceConnector(ctx context.Context, categoryID, connectorID string) (*GovernanceConnector, error) {
	unlock := c.LockTenantConfig()
	defer unlock()
	var out GovernanceConnector
	if err := c.Get(ctx, c.governancePath(categoryID, connectorID), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdateGovernanceConnector sets connector properties.
func (c *Client) UpdateGovernanceConnector(ctx context.Context, categoryID, connectorID string, props map[string]string) error {
	if len(props) == 0 {
		return nil
	}
	unlock := c.LockTenantConfig()
	defer unlock()
	list := make([]map[string]string, 0, len(props))
	for k, v := range props {
		list = append(list, map[string]string{"name": k, "value": v})
	}
	_, err := c.Patch(ctx, c.governancePath(categoryID, connectorID), map[string]any{"operation": "UPDATE", "properties": list}, nil)
	return err
}

// RevertGovernanceProperties resets properties of a connector to the server defaults.
func (c *Client) RevertGovernanceProperties(ctx context.Context, categoryID, connectorID string, names []string) error {
	if len(names) == 0 {
		return nil
	}
	unlock := c.LockTenantConfig()
	defer unlock()
	_, err := c.Do(ctx, Request{
		Method: http.MethodPost,
		Path:   c.ServerPath("/identity-governance/" + url.PathEscape(categoryID) + "/connectors/revert"),
		Query:  url.Values{"connectorId": {connectorID}},
		Body:   map[string]any{"properties": names},
	})
	return err
}
