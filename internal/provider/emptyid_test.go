package provider_test

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"github.com/hayone1/terraform-provider-wso2is/internal/provider"
)

// Crossplane's Upjet reads an object before it creates it. For an object whose ID the
// server assigns, the ID in that read is empty. Every resource must then report the
// object as missing, without calling the server; otherwise Crossplane treats the object as
// existing and never creates it. Terraform itself never reads an object without an ID.
//
// The provider points at an address that cannot resolve, so a Read that calls the server
// fails the test with a connection error.
func TestResources_readWithoutIDIsNotFound(t *testing.T) {
	ctx := context.Background()
	server := providerserver.NewProtocol6(provider.New("test")())()

	schemas, err := server.GetProviderSchema(ctx, &tfprotov6.GetProviderSchemaRequest{})
	if err != nil {
		t.Fatal(err)
	}
	configureProvider(t, ctx, server, schemas.Provider)

	for name, s := range schemas.ResourceSchemas {
		for _, id := range []tftypes.Value{tftypes.NewValue(tftypes.String, ""), tftypes.NewValue(tftypes.String, nil)} {
			t.Run(name+"/id="+id.String(), func(t *testing.T) {
				objType := s.ValueType().(tftypes.Object)
				attrs := map[string]tftypes.Value{}
				for k, typ := range objType.AttributeTypes {
					attrs[k] = tftypes.NewValue(typ, nil)
				}
				// Required arguments get a value, as they do in a managed resource.
				for _, a := range s.Block.Attributes {
					if a.Required && objType.AttributeTypes[a.Name].Is(tftypes.String) {
						// "{}" is also valid for JSON arguments.
						attrs[a.Name] = tftypes.NewValue(tftypes.String, "{}")
					}
				}
				attrs["id"] = id
				state, err := tfprotov6.NewDynamicValue(objType, tftypes.NewValue(objType, attrs))
				if err != nil {
					t.Fatal(err)
				}
				resp, err := server.ReadResource(ctx, &tfprotov6.ReadResourceRequest{TypeName: name, CurrentState: &state})
				if err != nil {
					t.Fatal(err)
				}
				for _, d := range resp.Diagnostics {
					if d.Severity == tfprotov6.DiagnosticSeverityError {
						t.Fatalf("Read returned an error: %s: %s", d.Summary, d.Detail)
					}
				}
				if resp.NewState == nil {
					return
				}
				got, err := resp.NewState.Unmarshal(objType)
				if err != nil {
					t.Fatal(err)
				}
				if !got.IsNull() {
					t.Fatalf("Read kept the object in the state; want it removed")
				}
			})
		}
	}
}

func configureProvider(t *testing.T, ctx context.Context, server tfprotov6.ProviderServer, s *tfprotov6.Schema) {
	t.Helper()
	objType := s.ValueType().(tftypes.Object)
	attrs := map[string]tftypes.Value{}
	for k, typ := range objType.AttributeTypes {
		attrs[k] = tftypes.NewValue(typ, nil)
	}
	attrs["base_url"] = tftypes.NewValue(tftypes.String, "https://wso2is.invalid")
	attrs["username"] = tftypes.NewValue(tftypes.String, "admin")
	attrs["password"] = tftypes.NewValue(tftypes.String, "admin")
	attrs["max_retries"] = tftypes.NewValue(tftypes.Number, 0)
	cfg, err := tfprotov6.NewDynamicValue(objType, tftypes.NewValue(objType, attrs))
	if err != nil {
		t.Fatal(err)
	}
	resp, err := server.ConfigureProvider(ctx, &tfprotov6.ConfigureProviderRequest{Config: &cfg})
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range resp.Diagnostics {
		if d.Severity == tfprotov6.DiagnosticSeverityError {
			t.Fatalf("configure: %s: %s", d.Summary, d.Detail)
		}
	}
}
