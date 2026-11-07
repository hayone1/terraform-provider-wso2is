// Package sweep removes objects left behind by interrupted acceptance tests: everything
// whose name starts with acctest.NamePrefix ("tfacc"). Run it with `task sweep`
// (go test ./internal/sweep -sweep=all) against a server used only for testing.
package sweep

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/hayone1/terraform-provider-wso2is/internal/acctest"
	"github.com/hayone1/terraform-provider-wso2is/internal/client"
)

func TestMain(m *testing.M) { resource.TestMain(m) }

// marker is matched case-insensitively: some tests strip the dashes or upper-case names.
var marker = strings.TrimSuffix(acctest.NamePrefix, "-")

func isTestObject(name string) bool {
	return strings.Contains(strings.ToLower(name), marker)
}

// item is one object to delete.
type item struct{ id, name string }

// sweeper lists objects with list and deletes the test objects with del.
func sweeper(name string, deps []string, list func(context.Context, *client.Client) ([]item, error), del func(context.Context, *client.Client, string) error) {
	resource.AddTestSweepers(name, &resource.Sweeper{
		Name:         name,
		Dependencies: deps,
		F: func(string) error {
			c, err := acctest.EnvClient()
			if err != nil {
				return err
			}
			ctx := context.Background()
			items, err := list(ctx, c)
			if err != nil {
				return fmt.Errorf("listing %s: %w", name, err)
			}
			var failed []string
			for _, it := range items {
				if !isTestObject(it.name) {
					continue
				}
				log.Printf("[INFO] sweeping %s %q (%s)", name, it.name, it.id)
				if err := del(ctx, c, it.id); err != nil && !client.IsNotFound(err) {
					failed = append(failed, fmt.Sprintf("%s: %s", it.name, err))
				}
			}
			if len(failed) > 0 {
				return fmt.Errorf("sweeping %s: %s", name, strings.Join(failed, "; "))
			}
			return nil
		},
	})
}

// listJSON reads a list endpoint; key selects the array (empty for a bare array), and
// idKey/nameKey the fields of each element.
func listJSON(path string, query url.Values, key, idKey, nameKey string) func(context.Context, *client.Client) ([]item, error) {
	return func(ctx context.Context, c *client.Client) ([]item, error) {
		var raw any
		if err := c.Get(ctx, path, query, &raw); err != nil {
			return nil, err
		}
		arr, _ := raw.([]any)
		if key != "" {
			if m, ok := raw.(map[string]any); ok {
				arr, _ = m[key].([]any)
			}
		}
		out := make([]item, 0, len(arr))
		for _, e := range arr {
			m, _ := e.(map[string]any)
			id, _ := m[idKey].(string)
			name, _ := m[nameKey].(string)
			out = append(out, item{id: id, name: name})
		}
		return out, nil
	}
}

func server(c *client.Client, p string) string { return c.ServerPath(p) }
func scim(c *client.Client, p string) string   { return c.SCIMPath(p) }
func deletePath(p func(*client.Client, string) string) func(context.Context, *client.Client, string) error {
	return func(ctx context.Context, c *client.Client, id string) error { return c.Delete(ctx, p(c, id)) }
}

// lazy defers building a path until the client is known.
func lazy(build func(*client.Client) string, query url.Values, key, idKey, nameKey string) func(context.Context, *client.Client) ([]item, error) {
	return func(ctx context.Context, c *client.Client) ([]item, error) {
		return listJSON(build(c), query, key, idKey, nameKey)(ctx, c)
	}
}

func init() {
	limit := url.Values{"limit": {"500"}}

	// Workflows first: an association would queue the deletions below for approval.
	sweeper("wso2is_workflow", nil,
		lazy(func(c *client.Client) string { return server(c, "/workflows") }, limit, "workflows", "id", "name"),
		deletePath(func(c *client.Client, id string) string { return server(c, "/workflows/"+url.PathEscape(id)) }))

	sweeper("wso2is_application", []string{"wso2is_workflow"},
		lazy(func(c *client.Client) string { return server(c, "/applications") }, limit, "applications", "id", "name"),
		deletePath(func(c *client.Client, id string) string { return server(c, "/applications/"+url.PathEscape(id)) }))

	sweeper("wso2is_api_resource", []string{"wso2is_application"},
		lazy(func(c *client.Client) string { return server(c, "/api-resources") }, url.Values{"limit": {"100"}, "filter": {"name sw " + marker}}, "apiResources", "id", "name"),
		deletePath(func(c *client.Client, id string) string { return server(c, "/api-resources/"+url.PathEscape(id)) }))

	sweeper("wso2is_identity_provider", []string{"wso2is_application"},
		lazy(func(c *client.Client) string { return server(c, "/identity-providers") }, limit, "identityProviders", "id", "name"),
		func(ctx context.Context, c *client.Client, id string) error {
			return c.DeleteIdentityProvider(ctx, id, true)
		})

	sweeper("wso2is_user", []string{"wso2is_workflow"},
		lazy(func(c *client.Client) string { return scim(c, "/Users") }, url.Values{"count": {"500"}, "filter": {"userName co " + marker}}, "Resources", "id", "userName"),
		deletePath(func(c *client.Client, id string) string { return scim(c, "/Users/"+url.PathEscape(id)) }))

	sweeper("wso2is_group", []string{"wso2is_workflow"},
		lazy(func(c *client.Client) string { return scim(c, "/Groups") }, url.Values{"count": {"500"}, "filter": {"displayName co " + marker}}, "Resources", "id", "displayName"),
		deletePath(func(c *client.Client, id string) string { return scim(c, "/Groups/"+url.PathEscape(id)) }))

	sweeper("wso2is_role", []string{"wso2is_application"},
		lazy(func(c *client.Client) string { return scim(c, "/v2/Roles") }, url.Values{"count": {"500"}, "filter": {"displayName co " + marker}}, "Resources", "id", "displayName"),
		deletePath(func(c *client.Client, id string) string { return scim(c, "/v2/Roles/"+url.PathEscape(id)) }))

	sweeper("wso2is_organization", []string{"wso2is_application", "wso2is_user"},
		lazy(func(c *client.Client) string { return server(c, "/organizations") }, url.Values{"limit": {"100"}, "recursive": {"false"}}, "organizations", "id", "name"),
		deletePath(func(c *client.Client, id string) string { return server(c, "/organizations/"+url.PathEscape(id)) }))

	sweeper("wso2is_userstore", []string{"wso2is_user", "wso2is_group"},
		lazy(func(c *client.Client) string { return server(c, "/userstores") }, nil, "", "id", "name"),
		func(ctx context.Context, c *client.Client, id string) error { return c.DeleteUserstore(ctx, id) })

	sweeper("wso2is_tenant", nil,
		func(ctx context.Context, c *client.Client) ([]item, error) {
			return listJSON("/api/server/v1/tenants", url.Values{"limit": {"100"}}, "tenants", "id", "domain")(ctx, c)
		},
		func(ctx context.Context, c *client.Client, id string) error {
			if err := c.SetTenantActivated(ctx, id, false); err != nil && !client.IsNotFound(err) {
				return err
			}
			return c.DeleteTenant(ctx, id)
		})

	// External claims before the local claims they map to; custom dialects last.
	sweeper("wso2is_external_claim", []string{"wso2is_vc_template"},
		func(ctx context.Context, c *client.Client) ([]item, error) {
			dialects, err := c.ListClaimDialects(ctx)
			if err != nil {
				return nil, err
			}
			var out []item
			for _, d := range dialects {
				if d.ID == "local" {
					continue
				}
				claims, err := c.ListExternalClaims(ctx, d.ID)
				if err != nil {
					continue
				}
				for _, cl := range claims {
					out = append(out, item{id: d.ID + "/" + cl.ID, name: cl.ClaimURI + " " + cl.MappedLocalClaimURI})
				}
			}
			return out, nil
		},
		func(ctx context.Context, c *client.Client, id string) error {
			dialect, claim, _ := strings.Cut(id, "/")
			return c.DeleteExternalClaim(ctx, dialect, claim)
		})

	sweeper("wso2is_claim", []string{"wso2is_external_claim"},
		func(ctx context.Context, c *client.Client) ([]item, error) {
			claims, err := c.ListLocalClaims(ctx)
			if err != nil {
				return nil, err
			}
			out := make([]item, 0, len(claims))
			for _, cl := range claims {
				id, _ := cl["id"].(string)
				uri, _ := cl["claimURI"].(string)
				out = append(out, item{id: id, name: uri})
			}
			return out, nil
		},
		func(ctx context.Context, c *client.Client, id string) error { return c.DeleteLocalClaim(ctx, id) })

	sweeper("wso2is_claim_dialect", []string{"wso2is_external_claim"},
		func(ctx context.Context, c *client.Client) ([]item, error) {
			dialects, err := c.ListClaimDialects(ctx)
			if err != nil {
				return nil, err
			}
			out := make([]item, 0, len(dialects))
			for _, d := range dialects {
				if d.ID != "local" {
					out = append(out, item{id: d.ID, name: d.DialectURI})
				}
			}
			return out, nil
		},
		func(ctx context.Context, c *client.Client, id string) error { return c.DeleteClaimDialect(ctx, id) })

	sweeper("wso2is_oidc_scope", nil,
		lazy(func(c *client.Client) string { return server(c, "/oidc/scopes") }, nil, "", "name", "name"),
		func(ctx context.Context, c *client.Client, name string) error { return c.DeleteOIDCScope(ctx, name) })

	sweeper("wso2is_script_library", nil,
		lazy(func(c *client.Client) string { return server(c, "/script-libraries") }, limit, "scriptLibraries", "name", "name"),
		deletePath(func(c *client.Client, name string) string {
			return server(c, "/script-libraries/"+url.PathEscape(name))
		}))

	sweeper("wso2is_webhook", nil,
		lazy(func(c *client.Client) string { return server(c, "/webhooks") }, nil, "webhooks", "id", "name"),
		func(ctx context.Context, c *client.Client, id string) error {
			// Active webhooks must be deactivated before deletion.
			_, _ = c.Post(ctx, server(c, "/webhooks/"+url.PathEscape(id)+"/deactivate"), nil, nil)
			return c.Delete(ctx, server(c, "/webhooks/"+url.PathEscape(id)))
		})

	sweeper("wso2is_idv_provider", nil,
		lazy(func(c *client.Client) string { return server(c, "/idv-providers") }, limit, "identityVerificationProviders", "id", "name"),
		deletePath(func(c *client.Client, id string) string { return server(c, "/idv-providers/"+url.PathEscape(id)) }))

	sweeper("wso2is_vc_template", nil,
		lazy(func(c *client.Client) string { return server(c, "/vc-templates") }, nil, "VCTemplates", "id", "identifier"),
		deletePath(func(c *client.Client, id string) string { return server(c, "/vc-templates/"+url.PathEscape(id)) }))

	sweeper("wso2is_custom_authenticator", []string{"wso2is_application"},
		lazy(func(c *client.Client) string { return server(c, "/authenticators") }, nil, "", "id", "name"),
		deletePath(func(c *client.Client, id string) string {
			return server(c, "/authenticators/custom/"+url.PathEscape(id))
		}))

	for _, actionType := range []string{"preIssueAccessToken", "preIssueIdToken", "preUpdatePassword", "preUpdateProfile", "preRegistration"} {
		t := actionType
		list := lazy(func(c *client.Client) string { return server(c, "/actions/"+t) }, nil, "", "id", "name")
		sweeper("wso2is_action_"+t, nil,
			func(ctx context.Context, c *client.Client) ([]item, error) {
				items, err := list(ctx, c)
				if code := client.StatusCode(err); code == 400 || code == 501 {
					return nil, nil // action type not available on this server
				}
				return items, err
			},
			deletePath(func(c *client.Client, id string) string { return server(c, "/actions/"+t+"/"+url.PathEscape(id)) }))
	}
}
