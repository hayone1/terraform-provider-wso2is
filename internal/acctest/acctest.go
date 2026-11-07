// Package acctest holds helpers shared by the acceptance tests. Acceptance tests run
// against a real WSO2 Identity Server (TF_ACC=1) configured through the provider's
// WSO2IS_* environment variables; see CONTRIBUTING.md.
package acctest

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/hayone1/terraform-provider-wso2is/internal/client"
	"github.com/hayone1/terraform-provider-wso2is/internal/provider"
)

// NamePrefix marks objects created by acceptance tests, so sweepers can find leftovers.
const NamePrefix = "tfacc-"

func init() {
	// Register the in-process provider under its real address (hayone1/wso2is), however
	// the tests are started. Test configurations can then name the source, and OpenTofu
	// accepts the address (it rejects the framework's default, registry.terraform.io/-/wso2is).
	if os.Getenv("TF_ACC_PROVIDER_NAMESPACE") == "" {
		_ = os.Setenv("TF_ACC_PROVIDER_NAMESPACE", "hayone1")
	}
	if os.Getenv("TF_ACC_PROVIDER_HOST") == "" && strings.Contains(filepath.Base(os.Getenv("TF_ACC_TERRAFORM_PATH")), "tofu") {
		_ = os.Setenv("TF_ACC_PROVIDER_HOST", "registry.opentofu.org")
	}
}

// ProtoV6ProviderFactories starts the provider in-process for terraform-plugin-testing.
var ProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"wso2is": providerserver.NewProtocol6WithError(provider.New("test")()),
}

// ExternalProviders builds the provider binary and installs it for Terraform from a
// local filesystem mirror. Use it instead of ProtoV6ProviderFactories for configurations
// with several wso2is provider blocks: in-process, all provider blocks share one server,
// so the last one configured wins; Terraform starts one process per provider block for
// real binaries.
func ExternalProviders(t *testing.T) map[string]resource.ExternalProvider {
	t.Helper()
	const version = "0.0.1"
	root, err := exec.Command("go", "list", "-m", "-f", "{{.Dir}}").Output()
	if err != nil {
		t.Fatalf("locating the module: %s", err)
	}
	mirror := t.TempDir()
	target := filepath.Join(mirror, "registry.terraform.io", "hayone1", "wso2is", version, runtime.GOOS+"_"+runtime.GOARCH)
	build := exec.Command("go", "build", "-o", filepath.Join(target, "terraform-provider-wso2is_v"+version), ".")
	build.Dir = strings.TrimSpace(string(root))
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("building the provider: %s\n%s", err, out)
	}
	rc := filepath.Join(mirror, "terraformrc")
	cfg := fmt.Sprintf(`provider_installation {
  filesystem_mirror {
    path    = %q
    include = ["hayone1/wso2is"]
  }
  direct {
    exclude = ["hayone1/wso2is"]
  }
}
`, mirror)
	if err := os.WriteFile(rc, []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TF_CLI_CONFIG_FILE", rc)
	return map[string]resource.ExternalProvider{"wso2is": {Source: "hayone1/wso2is", VersionConstraint: version}}
}

// PreCheck skips the test unless TF_ACC is set, and fails fast with a clear message when
// the environment is not configured for acceptance tests.
func PreCheck(t *testing.T) {
	t.Helper()
	if os.Getenv("TF_ACC") == "" {
		t.Skip("acceptance tests are skipped unless TF_ACC is set")
	}
	if os.Getenv("WSO2IS_BASE_URL") == "" {
		t.Fatal("WSO2IS_BASE_URL must be set for acceptance tests (see CONTRIBUTING.md)")
	}
	hasBasic := os.Getenv("WSO2IS_USERNAME") != "" && os.Getenv("WSO2IS_PASSWORD") != ""
	hasClient := os.Getenv("WSO2IS_CLIENT_ID") != "" && os.Getenv("WSO2IS_CLIENT_SECRET") != ""
	hasToken := os.Getenv("WSO2IS_ACCESS_TOKEN") != ""
	if !hasBasic && !hasClient && !hasToken {
		t.Fatal("set WSO2IS_USERNAME/WSO2IS_PASSWORD, WSO2IS_CLIENT_ID/WSO2IS_CLIENT_SECRET or WSO2IS_ACCESS_TOKEN for acceptance tests")
	}
}

// RandomName returns a unique name with the acceptance test prefix.
func RandomName(kind string) string {
	return fmt.Sprintf("%s%s-%s", NamePrefix, kind, acctest.RandString(8))
}

var (
	apiOnce   sync.Once
	apiClient *client.Client
	apiErr    error
)

// Client returns an API client configured like the provider (from the environment), for
// out-of-band checks and setup in tests.
func Client(t *testing.T) *client.Client {
	t.Helper()
	apiOnce.Do(func() { apiClient, apiErr = client.New(envConfig()) })
	if apiErr != nil {
		t.Fatalf("creating API client: %s", apiErr)
	}
	return apiClient
}

// EnvClient returns an API client configured from the provider's environment variables,
// for code that runs outside a test (sweepers).
func EnvClient() (*client.Client, error) { return client.New(envConfig()) }

// TenantClient returns an API client for a tenant created by TestTenant.
func TenantClient(t *testing.T, domain, username, password string) *client.Client {
	t.Helper()
	cfg := envConfig()
	cfg.TenantDomain, cfg.Username, cfg.Password = domain, username, password
	cfg.ClientID, cfg.ClientSecret, cfg.AccessToken = "", "", ""
	c, err := client.New(cfg)
	if err != nil {
		t.Fatalf("creating tenant API client: %s", err)
	}
	return c
}

// envConfig reads the client settings from the provider's environment variables.
func envConfig() client.Config {
	cfg := client.Config{
		BaseURL:      os.Getenv("WSO2IS_BASE_URL"),
		TenantDomain: os.Getenv("WSO2IS_TENANT_DOMAIN"),
		Username:     os.Getenv("WSO2IS_USERNAME"),
		Password:     os.Getenv("WSO2IS_PASSWORD"),
		ClientID:     os.Getenv("WSO2IS_CLIENT_ID"),
		ClientSecret: os.Getenv("WSO2IS_CLIENT_SECRET"),
		AccessToken:  os.Getenv("WSO2IS_ACCESS_TOKEN"),
		Timeout:      60 * time.Second,
		MaxRetries:   4,
	}
	if s := os.Getenv("WSO2IS_INSECURE_SKIP_VERIFY"); s == "true" || s == "1" {
		cfg.InsecureSkipVerify = true
	}
	if s := os.Getenv("WSO2IS_SCOPES"); s != "" {
		cfg.Scopes = strings.Fields(s)
	}
	if ca := os.Getenv("WSO2IS_CA_CERTIFICATE"); ca != "" {
		if !strings.Contains(ca, "-----BEGIN") {
			if b, err := os.ReadFile(ca); err == nil {
				ca = string(b)
			}
		}
		cfg.CACertPEM = ca
	}
	return cfg
}

// Context returns a context for API calls made from tests.
func Context(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)
	return ctx
}

// CheckDestroyed returns a CheckDestroy function that fails when exists reports that an
// object of the given resource type is still present.
func CheckDestroyed(t *testing.T, resourceType string, exists func(ctx context.Context, c *client.Client, id string) (bool, error)) func(*terraform.State) error {
	return func(s *terraform.State) error {
		c := Client(t)
		for _, rs := range s.RootModule().Resources {
			if rs.Type != resourceType {
				continue
			}
			found, err := exists(context.Background(), c, rs.Primary.ID)
			if err != nil {
				return err
			}
			if found {
				return fmt.Errorf("%s %s still exists", resourceType, rs.Primary.ID)
			}
		}
		return nil
	}
}

// ResourceID returns the ID of a resource in state.
func ResourceID(s *terraform.State, address string) (string, error) {
	rs, ok := s.RootModule().Resources[address]
	if !ok {
		return "", fmt.Errorf("resource %s not found in state", address)
	}
	return rs.Primary.ID, nil
}

// DeleteOutOfBand runs del against the resource's ID; use it in a PreConfig or Check to
// simulate objects removed outside Terraform.
func DeleteOutOfBand(t *testing.T, address string, del func(ctx context.Context, c *client.Client, id string) error) func(*terraform.State) error {
	return func(s *terraform.State) error {
		id, err := ResourceID(s, address)
		if err != nil {
			return err
		}
		return del(context.Background(), Client(t), id)
	}
}

// TestTenant creates a tenant for tests that cannot run in the super tenant, waits until
// it serves API calls and deletes it when the test ends. It returns the domain and the
// owner's credentials.
func TestTenant(t *testing.T) (domain, username, password string) {
	t.Helper()
	c := Client(t)
	domain = strings.ToLower(RandomName("tenant")) + ".example"
	password = "Tenant-Owner-Passw0rd!"
	ctx := Context(t)
	id, err := c.CreateTenant(ctx, &client.Tenant{Domain: domain, Owners: []client.TenantOwner{{
		Username: "owner", Password: password, Email: "owner@" + domain, FirstName: "Test", LastName: "Owner", ProvisioningMethod: "inline-password",
	}}})
	if err != nil {
		t.Fatalf("creating test tenant: %s", err)
	}
	t.Cleanup(func() {
		ctx := context.Background()
		_ = c.SetTenantActivated(ctx, id, false)
		_ = c.DeleteTenant(ctx, id)
	})
	username = "owner@" + domain
	tc, err := client.New(client.Config{BaseURL: os.Getenv("WSO2IS_BASE_URL"), TenantDomain: domain, Username: username, Password: password, InsecureSkipVerify: os.Getenv("WSO2IS_INSECURE_SKIP_VERIFY") == "true", Timeout: 30 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(2 * time.Minute)
	for {
		var out any
		if err = tc.Get(ctx, tc.ServerPath("/applications"), nil, &out); err == nil {
			return domain, username, password
		}
		if time.Now().After(deadline) {
			t.Fatalf("test tenant %s not ready: %s", domain, err)
		}
		time.Sleep(3 * time.Second)
	}
}

// TestOrganization creates a sub-organization and an M2M application in the root
// organization that can manage it (client credentials + organization_switch, authorized
// for every organization API and shared with the organization). It returns the
// organization ID and the application's credentials, and removes both when the test ends.
func TestOrganization(t *testing.T) (orgID, clientID, clientSecret string) {
	t.Helper()
	c := Client(t)
	ctx := Context(t)
	name := RandomName("org")
	orgID, err := c.CreateOrganization(ctx, &client.Organization{Name: name})
	if err != nil {
		t.Fatalf("creating test organization: %s", err)
	}
	appID, err := c.CreateApplication(ctx, &client.Application{Name: name + "-m2m", TemplateID: strPtr("m2m-application"),
		InboundProtocolConfiguration: &client.InboundProtocols{OIDC: map[string]any{"grantTypes": []string{"client_credentials", "organization_switch"}}}})
	if err != nil {
		t.Fatalf("creating test M2M application: %s", err)
	}
	t.Cleanup(func() {
		ctx := context.Background()
		_ = c.DeleteApplication(ctx, appID)
		_ = c.DeleteOrganization(ctx, orgID)
	})
	apis, err := c.ListAPIResources(ctx, "type eq ORGANIZATION")
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range apis {
		if a.Type != "ORGANIZATION" {
			continue
		}
		full, err := c.GetAPIResource(ctx, a.ID)
		if err != nil {
			t.Fatal(err)
		}
		scopes := make([]string, 0, len(full.Scopes))
		for _, s := range full.Scopes {
			scopes = append(scopes, s.Name)
		}
		if err := c.AuthorizeAPI(ctx, appID, a.ID, "RBAC", scopes, nil); err != nil && !client.IsConflict(err) {
			t.Fatalf("authorizing %s: %s", a.Identifier, err)
		}
	}
	if err := c.ShareApplication(ctx, appID, false, []string{orgID}); err != nil {
		t.Fatalf("sharing test application: %s", err)
	}
	oidc, err := c.GetInboundOIDC(ctx, appID)
	if err != nil {
		t.Fatal(err)
	}
	clientID, _ = oidc["clientId"].(string)
	clientSecret, _ = oidc["clientSecret"].(string)
	return orgID, clientID, clientSecret
}

func strPtr(s string) *string { return &s }
