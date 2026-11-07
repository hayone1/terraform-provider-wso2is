package client

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
)

// SCIM schema URNs used by WSO2 Identity Server.
const (
	SCIMCoreUser       = "urn:ietf:params:scim:schemas:core:2.0:User"
	SCIMEnterpriseUser = "urn:ietf:params:scim:schemas:extension:enterprise:2.0:User"
	SCIMWSO2User       = "urn:scim:wso2:schema"
	SCIMCustomUser     = "urn:scim:schemas:extension:custom:User"
	SCIMPatchOp        = "urn:ietf:params:scim:api:messages:2.0:PatchOp"
	SCIMRoleV2         = "urn:ietf:params:scim:schemas:extension:2.0:Role"
	SCIMGroup          = "urn:ietf:params:scim:schemas:core:2.0:Group"
)

// SCIMPatchOperation is one operation of a SCIM PATCH request.
type SCIMPatchOperation struct {
	Op    string `json:"op"`
	Path  string `json:"path,omitempty"`
	Value any    `json:"value,omitempty"`
}

func patchBody(ops []SCIMPatchOperation) map[string]any {
	return map[string]any{"schemas": []string{SCIMPatchOp}, "Operations": ops}
}

// SCIMMember is a member reference (user or group).
type SCIMMember struct {
	Value   string `json:"value"`
	Display string `json:"display,omitempty"`
	Type    string `json:"type,omitempty"`
}

type scimList struct {
	TotalResults int              `json:"totalResults"`
	StartIndex   int              `json:"startIndex"`
	ItemsPerPage int              `json:"itemsPerPage"`
	Resources    []map[string]any `json:"Resources"`
}

// listSCIM pages through a SCIM list endpoint.
func (c *Client) listSCIM(ctx context.Context, p, filter, attributes string) ([]map[string]any, error) {
	var out []map[string]any
	const count = 100
	for start := 1; ; start += count {
		q := url.Values{"startIndex": {strconv.Itoa(start)}, "count": {strconv.Itoa(count)}}
		if filter != "" {
			q.Set("filter", filter)
		}
		if attributes != "" {
			q.Set("attributes", attributes)
		}
		var page scimList
		if err := c.Get(ctx, p, q, &page); err != nil {
			return nil, err
		}
		out = append(out, page.Resources...)
		if len(page.Resources) < count || len(out) >= page.TotalResults {
			return out, nil
		}
	}
}

// ---- Users ----

// CreateUser creates a user from a SCIM body and returns the created representation.
func (c *Client) CreateUser(ctx context.Context, body map[string]any) (map[string]any, error) {
	var out map[string]any
	_, err := c.Post(ctx, c.SCIMPath("/Users"), body, &out)
	return out, err
}

// GetUser reads a user.
func (c *Client) GetUser(ctx context.Context, id string) (map[string]any, error) {
	var out map[string]any
	if err := c.Get(ctx, c.SCIMPath("/Users/"+url.PathEscape(id)), nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// PatchUser applies SCIM PATCH operations to a user.
func (c *Client) PatchUser(ctx context.Context, id string, ops []SCIMPatchOperation) error {
	if len(ops) == 0 {
		return nil
	}
	_, err := c.Patch(ctx, c.SCIMPath("/Users/"+url.PathEscape(id)), patchBody(ops), nil)
	return err
}

// DeleteUser deletes a user.
func (c *Client) DeleteUser(ctx context.Context, id string) error {
	return c.Delete(ctx, c.SCIMPath("/Users/"+url.PathEscape(id)))
}

// FindUsers returns users matching a SCIM filter, for example `userName eq alice`.
func (c *Client) FindUsers(ctx context.Context, filter string) ([]map[string]any, error) {
	return c.listSCIM(ctx, c.SCIMPath("/Users"), filter, "")
}

// ---- Groups ----

// Group is a SCIM group.
type Group struct {
	ID          string       `json:"id,omitempty"`
	DisplayName string       `json:"displayName"`
	Members     []SCIMMember `json:"members,omitempty"`
	Schemas     []string     `json:"schemas,omitempty"`
}

// CreateGroup creates a group. displayName carries the user store prefix
// (`SECONDARY/devs`) for groups outside the primary user store.
func (c *Client) CreateGroup(ctx context.Context, displayName string, members []SCIMMember) (*Group, error) {
	var out Group
	body := Group{DisplayName: displayName, Members: members, Schemas: []string{SCIMGroup}}
	if _, err := c.Post(ctx, c.SCIMPath("/Groups"), body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetGroup reads a group with its members.
func (c *Client) GetGroup(ctx context.Context, id string) (*Group, error) {
	var out Group
	if err := c.Get(ctx, c.SCIMPath("/Groups/"+url.PathEscape(id)), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// PatchGroup applies SCIM PATCH operations to a group.
func (c *Client) PatchGroup(ctx context.Context, id string, ops []SCIMPatchOperation) error {
	if len(ops) == 0 {
		return nil
	}
	_, err := c.Patch(ctx, c.SCIMPath("/Groups/"+url.PathEscape(id)), patchBody(ops), nil)
	return err
}

// DeleteGroup deletes a group.
func (c *Client) DeleteGroup(ctx context.Context, id string) error {
	return c.Delete(ctx, c.SCIMPath("/Groups/"+url.PathEscape(id)))
}

// FindGroups returns groups matching a SCIM filter.
func (c *Client) FindGroups(ctx context.Context, filter string) ([]Group, error) {
	raw, err := c.listSCIM(ctx, c.SCIMPath("/Groups"), filter, "")
	if err != nil {
		return nil, err
	}
	out := make([]Group, 0, len(raw))
	for _, r := range raw {
		var g Group
		if err := FromMap(r, &g); err == nil {
			out = append(out, g)
		}
	}
	return out, nil
}

// ---- Roles (v2) ----

// Role is a SCIM v2 role.
type Role struct {
	ID          string           `json:"id,omitempty"`
	DisplayName string           `json:"displayName"`
	Audience    *RoleAudience    `json:"audience,omitempty"`
	Permissions []RolePermission `json:"permissions,omitempty"`
	Users       []SCIMMember     `json:"users,omitempty"`
	Groups      []SCIMMember     `json:"groups,omitempty"`
	Schemas     []string         `json:"schemas,omitempty"`
	Properties  []struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	} `json:"properties,omitempty"`
}

// RoleAudience is the audience of a role: `organization` or `application`.
type RoleAudience struct {
	Type    string `json:"type"`
	Value   string `json:"value,omitempty"`
	Display string `json:"display,omitempty"`
}

// RolePermission is a scope granted by a role.
type RolePermission struct {
	Value   string `json:"value"`
	Display string `json:"display,omitempty"`
}

// CreateRole creates a role.
func (c *Client) CreateRole(ctx context.Context, r *Role) (*Role, error) {
	r.Schemas = []string{SCIMRoleV2}
	var out Role
	if _, err := c.Post(ctx, c.SCIMPath("/v2/Roles"), r, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetRole reads a role.
func (c *Client) GetRole(ctx context.Context, id string) (*Role, error) {
	var out Role
	if err := c.Get(ctx, c.SCIMPath("/v2/Roles/"+url.PathEscape(id)), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// PatchRole applies SCIM PATCH operations to a role. PUT is never used: it drops the
// role's user and group assignments.
func (c *Client) PatchRole(ctx context.Context, id string, ops []SCIMPatchOperation) error {
	if len(ops) == 0 {
		return nil
	}
	_, err := c.Patch(ctx, c.SCIMPath("/v2/Roles/"+url.PathEscape(id)), patchBody(ops), nil)
	return err
}

// DeleteRole deletes a role.
func (c *Client) DeleteRole(ctx context.Context, id string) error {
	return c.Delete(ctx, c.SCIMPath("/v2/Roles/"+url.PathEscape(id)))
}

// FindRoles returns roles matching a SCIM filter, for example `displayName eq admin`.
func (c *Client) FindRoles(ctx context.Context, filter string) ([]Role, error) {
	raw, err := c.listSCIM(ctx, c.SCIMPath("/v2/Roles"), filter, "")
	if err != nil {
		return nil, err
	}
	out := make([]Role, 0, len(raw))
	for _, r := range raw {
		var role Role
		if err := FromMap(r, &role); err == nil {
			out = append(out, role)
		}
	}
	return out, nil
}

// ---- Schemas ----

// SCIMAttributeType is the type of a SCIM attribute: string, boolean, integer,
// decimal, dateTime, complex, ...
type SCIMAttributeType struct {
	Type        string
	MultiValued bool
}

var (
	schemaMu    sync.Mutex
	schemaCache = map[string]map[string]SCIMAttributeType{}
)

// SCIMUserAttributeTypes returns the user attribute types keyed by full path
// (`urn:scim:wso2:schema:accountLocked`, `name.givenName`). The result is cached per
// server and tenant.
func (c *Client) SCIMUserAttributeTypes(ctx context.Context) (map[string]SCIMAttributeType, error) {
	key := c.BaseURL() + c.tenantPrefix()
	schemaMu.Lock()
	defer schemaMu.Unlock()
	if m, ok := schemaCache[key]; ok {
		return m, nil
	}
	var schemas []struct {
		ID         string           `json:"id"`
		Attributes []map[string]any `json:"attributes"`
	}
	if err := c.Get(ctx, c.SCIMPath("/Schemas"), nil, &schemas); err != nil {
		return nil, err
	}
	out := map[string]SCIMAttributeType{}
	var walk func(prefix string, attrs []map[string]any)
	walk = func(prefix string, attrs []map[string]any) {
		for _, a := range attrs {
			name, _ := a["name"].(string)
			typ, _ := a["type"].(string)
			mv, _ := a["multiValued"].(bool)
			p := prefix + name
			out[p] = SCIMAttributeType{Type: typ, MultiValued: mv}
			if subs, ok := a["subAttributes"].([]any); ok {
				var sub []map[string]any
				for _, s := range subs {
					if m, ok := s.(map[string]any); ok {
						sub = append(sub, m)
					}
				}
				walk(p+".", sub)
			}
		}
	}
	for _, s := range schemas {
		switch s.ID {
		case SCIMCoreUser:
			walk("", s.Attributes)
		case "urn:ietf:params:scim:schemas:core:2.0":
			// Common attributes (id, meta); not settable.
		default:
			walk(s.ID+":", s.Attributes)
		}
	}
	schemaCache[key] = out
	return out, nil
}

// SplitSCIMPath splits `urn:scim:wso2:schema:accountLocked` into the schema URN and the
// attribute path; core attributes (`nickName`, `name.middleName`) have no schema.
func SplitSCIMPath(p string) (schema, attr string) {
	if !strings.HasPrefix(p, "urn:") {
		return "", p
	}
	i := strings.LastIndex(p, ":")
	return p[:i], p[i+1:]
}

// SCIMValue converts a string to the JSON type of the attribute.
func SCIMValue(types map[string]SCIMAttributeType, p, v string) (any, error) {
	t, ok := types[p]
	if !ok {
		return v, nil
	}
	switch strings.ToLower(t.Type) {
	case "boolean":
		b, err := strconv.ParseBool(v)
		if err != nil {
			return nil, fmt.Errorf("attribute %s is a boolean, got %q", p, v)
		}
		return b, nil
	case "integer":
		i, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("attribute %s is an integer, got %q", p, v)
		}
		return i, nil
	case "decimal":
		f, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return nil, fmt.Errorf("attribute %s is a decimal, got %q", p, v)
		}
		return f, nil
	}
	if t.MultiValued {
		parts := strings.Split(v, ",")
		for i := range parts {
			parts[i] = strings.TrimSpace(parts[i])
		}
		return parts, nil
	}
	return v, nil
}

// SCIMString renders a JSON value read from SCIM as the string form used in state.
func SCIMString(v any) (string, bool) {
	switch t := v.(type) {
	case nil:
		return "", false
	case string:
		return t, true
	case bool:
		return strconv.FormatBool(t), true
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64), true
	case []any:
		parts := make([]string, 0, len(t))
		for _, e := range t {
			if s, ok := SCIMString(e); ok {
				parts = append(parts, s)
			}
		}
		return strings.Join(parts, ","), true
	}
	return fmt.Sprint(v), true
}

// LookupSCIM returns the value at a SCIM path in a resource representation.
func LookupSCIM(res map[string]any, p string) (any, bool) {
	schema, attr := SplitSCIMPath(p)
	cur := res
	if schema != "" {
		m, ok := res[schema].(map[string]any)
		if !ok {
			return nil, false
		}
		cur = m
	}
	parts := strings.Split(attr, ".")
	for i, part := range parts {
		v, ok := cur[part]
		if !ok {
			return nil, false
		}
		if i == len(parts)-1 {
			return v, true
		}
		m, ok := v.(map[string]any)
		if !ok {
			return nil, false
		}
		cur = m
	}
	return nil, false
}

// SetSCIM sets the value at a SCIM path in a resource body, creating objects as needed.
func SetSCIM(res map[string]any, p string, v any) {
	schema, attr := SplitSCIMPath(p)
	cur := res
	if schema != "" {
		m, ok := res[schema].(map[string]any)
		if !ok {
			m = map[string]any{}
			res[schema] = m
		}
		cur = m
	}
	parts := strings.Split(attr, ".")
	for i, part := range parts {
		if i == len(parts)-1 {
			cur[part] = v
			return
		}
		m, ok := cur[part].(map[string]any)
		if !ok {
			m = map[string]any{}
			cur[part] = m
		}
		cur = m
	}
}

// IsSCIMNotFound reports a 404 from SCIM endpoints, which some versions return as 400
// with "does not exist".
func IsSCIMNotFound(err error) bool {
	if IsNotFound(err) {
		return true
	}
	var apiErr *APIError
	if StatusCode(err) == http.StatusBadRequest && errorAs(err, &apiErr) {
		return strings.Contains(strings.ToLower(apiErr.Message), "does not exist") || strings.Contains(strings.ToLower(apiErr.Message), "not found")
	}
	return false
}
