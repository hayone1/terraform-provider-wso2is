package provider_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"github.com/hayone1/terraform-provider-wso2is/internal/provider"
)

// Crossplane deletes all managed resources of a manifest at the same time. A group or a role
// can therefore disappear between the read and the PATCH that removes its members. The
// PATCH then answers 404, and the delete must still succeed: the members are gone with the
// group or role. Found 2026-11-07 in the Crossplane provider's end-to-end tests.
func TestMembers_deleteWhenParentDisappears(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/scim2/Groups/g1"):
			_, _ = w.Write([]byte(`{"id":"g1","displayName":"devs","members":[{"value":"u1","display":"alice"}]}`))
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/scim2/v2/Roles/r1"):
			_, _ = w.Write([]byte(`{"id":"r1","displayName":"support","users":[{"value":"u1"}],"groups":[{"value":"g1"}]}`))
		case r.Method == http.MethodPatch:
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"schemas":["urn:ietf:params:scim:api:messages:2.0:Error"],"status":"404","detail":"Group not found for ID: g1"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)

	ctx := context.Background()
	server := providerserver.NewProtocol6(provider.New("test")())()
	schemas, err := server.GetProviderSchema(ctx, &tfprotov6.GetProviderSchemaRequest{})
	if err != nil {
		t.Fatal(err)
	}
	pType := schemas.Provider.ValueType().(tftypes.Object)
	pAttrs := map[string]tftypes.Value{}
	for k, typ := range pType.AttributeTypes {
		pAttrs[k] = tftypes.NewValue(typ, nil)
	}
	pAttrs["base_url"] = tftypes.NewValue(tftypes.String, srv.URL)
	pAttrs["username"] = tftypes.NewValue(tftypes.String, "admin")
	pAttrs["password"] = tftypes.NewValue(tftypes.String, "admin")
	pAttrs["insecure_skip_verify"] = tftypes.NewValue(tftypes.Bool, true)
	pAttrs["max_retries"] = tftypes.NewValue(tftypes.Number, 0)
	cfg, _ := tfprotov6.NewDynamicValue(pType, tftypes.NewValue(pType, pAttrs))
	if resp, err := server.ConfigureProvider(ctx, &tfprotov6.ConfigureProviderRequest{Config: &cfg}); err != nil || len(resp.Diagnostics) > 0 {
		t.Fatalf("configure: %v %v", err, resp.Diagnostics)
	}

	set := func(ids ...string) tftypes.Value {
		vals := make([]tftypes.Value, 0, len(ids))
		for _, id := range ids {
			vals = append(vals, tftypes.NewValue(tftypes.String, id))
		}
		return tftypes.NewValue(tftypes.Set{ElementType: tftypes.String}, vals)
	}
	for name, state := range map[string]map[string]tftypes.Value{
		"wso2is_group_members":   {"id": tftypes.NewValue(tftypes.String, "g1"), "group_id": tftypes.NewValue(tftypes.String, "g1"), "user_ids": set("u1")},
		"wso2is_role_assignment": {"id": tftypes.NewValue(tftypes.String, "r1"), "role_id": tftypes.NewValue(tftypes.String, "r1"), "user_ids": set("u1"), "group_ids": set("g1")},
	} {
		t.Run(name, func(t *testing.T) {
			objType := schemas.ResourceSchemas[name].ValueType().(tftypes.Object)
			prior, err := tfprotov6.NewDynamicValue(objType, tftypes.NewValue(objType, state))
			if err != nil {
				t.Fatal(err)
			}
			null, _ := tfprotov6.NewDynamicValue(objType, tftypes.NewValue(objType, nil))
			resp, err := server.ApplyResourceChange(ctx, &tfprotov6.ApplyResourceChangeRequest{
				TypeName: name, PriorState: &prior, PlannedState: &null, Config: &null,
			})
			if err != nil {
				t.Fatal(err)
			}
			for _, d := range resp.Diagnostics {
				if d.Severity == tfprotov6.DiagnosticSeverityError {
					t.Fatalf("delete failed: %s: %s", d.Summary, d.Detail)
				}
			}
		})
	}
}
