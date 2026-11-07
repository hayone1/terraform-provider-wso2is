package client

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
)

// TenantOwner is the administrator account created with a tenant.
type TenantOwner struct {
	ID                 string              `json:"id,omitempty"`
	Username           string              `json:"username,omitempty"`
	Password           string              `json:"password,omitempty"`
	Email              string              `json:"email,omitempty"`
	FirstName          string              `json:"firstname,omitempty"`
	LastName           string              `json:"lastname,omitempty"`
	ProvisioningMethod string              `json:"provisioningMethod,omitempty"`
	AdditionalClaims   []map[string]string `json:"additionalClaims,omitempty"`
}

// Tenant is a tenant (root organization).
type Tenant struct {
	ID              string        `json:"id,omitempty"`
	Name            string        `json:"name,omitempty"`
	Domain          string        `json:"domain"`
	Owners          []TenantOwner `json:"owners"`
	CreatedDate     string        `json:"createdDate,omitempty"`
	LifecycleStatus *struct {
		Activated bool `json:"activated"`
	} `json:"lifecycleStatus,omitempty"`
}

// Tenant management is a super tenant API without the /t/{tenant} prefix.
func tenantsPath(p string) string { return "/api/server/v1/tenants" + p }

// CreateTenant creates a tenant and returns its ID.
func (c *Client) CreateTenant(ctx context.Context, t *Tenant) (string, error) {
	resp, err := c.Post(ctx, tenantsPath(""), t, nil)
	if err != nil {
		return "", err
	}
	if id := resp.LocationID(); id != "" {
		return id, nil
	}
	return "", fmt.Errorf("tenant created but the server returned no location")
}

// GetTenant reads a tenant by ID.
func (c *Client) GetTenant(ctx context.Context, id string) (*Tenant, error) {
	var out Tenant
	if err := c.Get(ctx, tenantsPath("/"+url.PathEscape(id)), nil, &out); err != nil {
		return nil, tenantNotFound(err)
	}
	return &out, nil
}

// tenantNotFound turns the tenant API's "Tenant cannot be found" (HTTP 400, TM-60014)
// into a 404.
func tenantNotFound(err error) error {
	var apiErr *APIError
	if errorAs(err, &apiErr) && (apiErr.Code == "TM-60014" || apiErr.Code == "TM-60009") {
		e := *apiErr
		e.StatusCode = http.StatusNotFound
		return &e
	}
	return err
}

// GetTenantByDomain reads a tenant by domain.
func (c *Client) GetTenantByDomain(ctx context.Context, domain string) (*Tenant, error) {
	var out Tenant
	if err := c.Get(ctx, tenantsPath("/domain/"+url.PathEscape(domain)), nil, &out); err != nil {
		return nil, tenantNotFound(err)
	}
	return &out, nil
}

// GetTenantOwner reads a tenant owner.
func (c *Client) GetTenantOwner(ctx context.Context, tenantID, ownerID string) (map[string]any, error) {
	var out map[string]any
	if err := c.Get(ctx, tenantsPath("/"+url.PathEscape(tenantID)+"/owners/"+url.PathEscape(ownerID)), nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// UpdateTenantOwner updates the owner's email, names or password.
func (c *Client) UpdateTenantOwner(ctx context.Context, tenantID, ownerID string, body map[string]any) error {
	_, err := c.Put(ctx, tenantsPath("/"+url.PathEscape(tenantID)+"/owners/"+url.PathEscape(ownerID)), body, nil)
	return err
}

// SetTenantActivated activates or deactivates a tenant.
func (c *Client) SetTenantActivated(ctx context.Context, id string, activated bool) error {
	_, err := c.Put(ctx, tenantsPath("/"+url.PathEscape(id)+"/lifecycle-status"), map[string]bool{"activated": activated}, nil)
	return tenantNotFound(err)
}

// DeleteTenant deletes a tenant's metadata (needs `[tenant_mgt] tenant_deletion = true`).
func (c *Client) DeleteTenant(ctx context.Context, id string) error {
	return tenantNotFound(c.Delete(ctx, tenantsPath("/"+url.PathEscape(id)+"/metadata")))
}
