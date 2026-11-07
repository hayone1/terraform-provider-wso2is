package provider_test

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"

	"github.com/hayone1/terraform-provider-wso2is/provider"
)

// An embedding program (such as Upjet) reads the schemas through the plugin protocol. The
// schemas must be valid, and the provider must offer every resource and data source.
func TestNew_servesValidSchemas(t *testing.T) {
	server := providerserver.NewProtocol6(provider.New("test")())()
	resp, err := server.GetProviderSchema(context.Background(), &tfprotov6.GetProviderSchemaRequest{})
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range resp.Diagnostics {
		if d.Severity == tfprotov6.DiagnosticSeverityError {
			t.Errorf("schema error: %s: %s", d.Summary, d.Detail)
		}
	}
	if got, want := len(resp.ResourceSchemas), 52; got != want {
		t.Errorf("resources: got %d, want %d (update this test and the Crossplane handoff when you add one)", got, want)
	}
	if got, want := len(resp.DataSourceSchemas), 12; got != want {
		t.Errorf("data sources: got %d, want %d", got, want)
	}
	for name := range resp.ResourceSchemas {
		if len(name) < len("wso2is_") || name[:len("wso2is_")] != "wso2is_" {
			t.Errorf("resource %q does not use the wso2is_ prefix", name)
		}
	}
}
