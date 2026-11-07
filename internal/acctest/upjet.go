package acctest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"testing"

	tfjson "github.com/hashicorp/terraform-json"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"

	"github.com/hayone1/terraform-provider-wso2is/internal/provider"
)

// TestOption changes how Test runs a test case.
type TestOption func(*testOptions)

type testOptions struct {
	skipUpjetRead  bool
	providerConfig map[string]any
}

// SkipUpjetRead turns the Upjet read check off. Use it only for test cases whose
// resources belong to more than one provider configuration, because the check reads every
// resource with one configuration.
func SkipUpjetRead() TestOption {
	return func(o *testOptions) { o.skipUpjetRead = true }
}

// UpjetReadProviderConfig sets the provider arguments that the Upjet read check uses. Pass
// the same values as the test configuration's provider block, for example the tenant
// domain and credentials from TestTenant. Without this option the check configures the
// provider from the WSO2IS_* environment variables only.
func UpjetReadProviderConfig(cfg map[string]any) TestOption {
	return func(o *testOptions) { o.providerConfig = cfg }
}

// Test runs an acceptance test case like resource.Test. It also adds the Upjet read check
// to every step that applies a configuration.
//
// Crossplane's Upjet runs this provider inside a Crossplane provider. When Upjet imports
// an existing object, or has lost its cached state, it does not call ImportState. It builds
// the prior state from the object's ID (the external name) and the arguments in the
// managed resource's spec, and calls Read. Computed attributes are empty. The check does
// the same for every wso2is resource in the state after each step: it clears every
// computed attribute except id, and calls ReadResource through the plugin protocol. It
// fails when Read returns an error, reports the object as missing, or does not restore a
// computed attribute. It logs, with the prefix "upjet read:", every argument that Read did
// not restore to its value in the state. Those are expected for optional arguments that
// the provider tracks only when they are set, or that the server does not return.
func Test(t *testing.T, tc resource.TestCase, opts ...TestOption) {
	t.Helper()
	var o testOptions
	for _, opt := range opts {
		opt(&o)
	}
	if !o.skipUpjetRead {
		for i := range tc.Steps {
			s := &tc.Steps[i]
			// A step that expects a non-empty plan usually changes or deletes objects outside
			// Terraform in its Check, which runs before the state checks.
			if s.ImportState || s.Destroy || s.PlanOnly || s.ExpectError != nil || s.ExpectNonEmptyPlan || (s.Config == "" && s.ConfigDirectory == nil && s.ConfigFile == nil) {
				continue
			}
			s.ConfigStateChecks = append(s.ConfigStateChecks, upjetReadCheck{t: t, providerConfig: o.providerConfig})
		}
	}
	resource.Test(t, tc)
}

type upjetReadCheck struct {
	t              *testing.T
	providerConfig map[string]any
}

var (
	protocolSchemaOnce sync.Once
	protocolSchema     *tfprotov6.GetProviderSchemaResponse
	protocolSchemaErr  error
)

func providerSchemas(ctx context.Context) (*tfprotov6.GetProviderSchemaResponse, error) {
	protocolSchemaOnce.Do(func() {
		server := providerserver.NewProtocol6(provider.New("test")())()
		protocolSchema, protocolSchemaErr = server.GetProviderSchema(ctx, &tfprotov6.GetProviderSchemaRequest{})
		if protocolSchemaErr == nil {
			protocolSchemaErr = diagnosticsError(protocolSchema.Diagnostics)
		}
	})
	return protocolSchema, protocolSchemaErr
}

// CheckState implements statecheck.StateCheck.
func (c upjetReadCheck) CheckState(ctx context.Context, req statecheck.CheckStateRequest, resp *statecheck.CheckStateResponse) {
	if req.State == nil || req.State.Values == nil || req.State.Values.RootModule == nil {
		return
	}
	schemas, err := providerSchemas(ctx)
	if err != nil {
		resp.Error = fmt.Errorf("upjet read: provider schema: %w", err)
		return
	}
	server, err := c.configuredServer(ctx, schemas)
	if err != nil {
		resp.Error = fmt.Errorf("upjet read: configuring the provider: %w", err)
		return
	}
	var errs []error
	for _, r := range managedResources(req.State.Values.RootModule) {
		sch, ok := schemas.ResourceSchemas[r.Type]
		if !ok || !strings.HasSuffix(r.ProviderName, "/wso2is") {
			continue
		}
		if err := c.read(ctx, server, sch, r); err != nil {
			errs = append(errs, fmt.Errorf("upjet read of %s: %w", r.Address, err))
		}
	}
	resp.Error = errors.Join(errs...)
}

func (c upjetReadCheck) configuredServer(ctx context.Context, schemas *tfprotov6.GetProviderSchemaResponse) (tfprotov6.ProviderServer, error) {
	server := providerserver.NewProtocol6(provider.New("test")())()
	typ := schemas.Provider.ValueType()
	b, err := json.Marshal(c.providerConfig)
	if err != nil {
		return nil, err
	}
	if c.providerConfig == nil {
		b = []byte("{}")
	}
	v, err := tftypes.ValueFromJSONWithOpts(b, typ, tftypes.ValueFromJSONOpts{})
	if err != nil {
		return nil, err
	}
	dv, err := tfprotov6.NewDynamicValue(typ, v)
	if err != nil {
		return nil, err
	}
	resp, err := server.ConfigureProvider(ctx, &tfprotov6.ConfigureProviderRequest{TerraformVersion: "upjet-read-check", Config: &dv})
	if err != nil {
		return nil, err
	}
	return server, diagnosticsError(resp.Diagnostics)
}

func (c upjetReadCheck) read(ctx context.Context, server tfprotov6.ProviderServer, sch *tfprotov6.Schema, r *tfjson.StateResource) error {
	typ := sch.ValueType()
	stateJSON, err := json.Marshal(r.AttributeValues)
	if err != nil {
		return err
	}
	inState, err := tftypes.ValueFromJSONWithOpts(stateJSON, typ, tftypes.ValueFromJSONOpts{})
	if err != nil {
		return fmt.Errorf("decoding the state: %w", err)
	}

	// The prior state Upjet builds when it imports an object: the ID and the arguments.
	var prior map[string]any
	if err := json.Unmarshal(stateJSON, &prior); err != nil {
		return err
	}
	clearComputed(sch.Block, prior, true)
	priorJSON, err := json.Marshal(prior)
	if err != nil {
		return err
	}
	priorValue, err := tftypes.ValueFromJSONWithOpts(priorJSON, typ, tftypes.ValueFromJSONOpts{})
	if err != nil {
		return fmt.Errorf("building the prior state: %w", err)
	}
	dv, err := tfprotov6.NewDynamicValue(typ, priorValue)
	if err != nil {
		return err
	}

	resp, err := server.ReadResource(ctx, &tfprotov6.ReadResourceRequest{TypeName: r.Type, CurrentState: &dv})
	if err != nil {
		return err
	}
	if err := diagnosticsError(resp.Diagnostics); err != nil {
		return fmt.Errorf("read failed: %w", err)
	}
	if resp.NewState == nil {
		return errors.New("read returned no state")
	}
	got, err := resp.NewState.Unmarshal(typ)
	if err != nil {
		return err
	}
	if got.IsNull() {
		return fmt.Errorf("read reported the object as missing; it needs one of the computed attributes to find the object (prior state: %s)", priorJSON)
	}

	diffs, err := inState.Diff(got)
	if err != nil {
		return err
	}
	changed := map[string]bool{}
	for _, d := range diffs {
		if d.Path == nil || len(d.Path.Steps()) == 0 {
			continue
		}
		if name, ok := d.Path.Steps()[0].(tftypes.AttributeName); ok {
			changed[string(name)] = true
		}
	}
	names := make([]string, 0, len(changed))
	for n := range changed {
		names = append(names, n)
	}
	sort.Strings(names)
	var stateAttrs, readAttrs map[string]tftypes.Value
	if err := inState.As(&stateAttrs); err != nil {
		return err
	}
	if err := got.As(&readAttrs); err != nil {
		return err
	}
	var errs []error
	for _, n := range names {
		kind := attributeKind(sch.Block, n)
		if kind == "computed" {
			// Upjet cannot fill a computed attribute from the spec, so Read must restore it.
			errs = append(errs, fmt.Errorf("computed attribute %s not restored: state %s, read %s", n, stateAttrs[n], readAttrs[n]))
			continue
		}
		c.t.Logf("upjet read: %s.%s (%s) not restored: state %s, read %s", r.Address, n, kind, stateAttrs[n], readAttrs[n])
	}
	return errors.Join(errs...)
}

// clearComputed sets every computed attribute in v to null, except a top-level id.
func clearComputed(block *tfprotov6.SchemaBlock, v map[string]any, top bool) {
	if block == nil || v == nil {
		return
	}
	for _, a := range block.Attributes {
		if top && a.Name == "id" {
			continue
		}
		if a.Computed {
			v[a.Name] = nil
			continue
		}
		if a.NestedType != nil {
			clearNested(a.NestedType, v[a.Name])
		}
	}
	for _, b := range block.BlockTypes {
		switch b.Nesting {
		case tfprotov6.SchemaNestedBlockNestingModeSingle, tfprotov6.SchemaNestedBlockNestingModeGroup:
			if m, ok := v[b.TypeName].(map[string]any); ok {
				clearComputed(b.Block, m, false)
			}
		default:
			if l, ok := v[b.TypeName].([]any); ok {
				for _, e := range l {
					if m, ok := e.(map[string]any); ok {
						clearComputed(b.Block, m, false)
					}
				}
			}
		}
	}
}

func clearNested(obj *tfprotov6.SchemaObject, v any) {
	block := &tfprotov6.SchemaBlock{Attributes: obj.Attributes}
	switch obj.Nesting {
	case tfprotov6.SchemaObjectNestingModeSingle:
		if m, ok := v.(map[string]any); ok {
			clearComputed(block, m, false)
		}
	case tfprotov6.SchemaObjectNestingModeList, tfprotov6.SchemaObjectNestingModeSet:
		if l, ok := v.([]any); ok {
			for _, e := range l {
				if m, ok := e.(map[string]any); ok {
					clearComputed(block, m, false)
				}
			}
		}
	case tfprotov6.SchemaObjectNestingModeMap:
		if mm, ok := v.(map[string]any); ok {
			for _, e := range mm {
				if m, ok := e.(map[string]any); ok {
					clearComputed(block, m, false)
				}
			}
		}
	}
}

func attributeKind(block *tfprotov6.SchemaBlock, name string) string {
	for _, a := range block.Attributes {
		if a.Name != name {
			continue
		}
		switch {
		case a.Computed && a.Optional:
			return "optional, computed"
		case a.Computed:
			return "computed"
		case a.Required:
			return "required"
		default:
			return "optional"
		}
	}
	return "block"
}

func managedResources(m *tfjson.StateModule) []*tfjson.StateResource {
	var out []*tfjson.StateResource
	for _, r := range m.Resources {
		if r.Mode == tfjson.ManagedResourceMode {
			out = append(out, r)
		}
	}
	for _, child := range m.ChildModules {
		out = append(out, managedResources(child)...)
	}
	return out
}

func diagnosticsError(diags []*tfprotov6.Diagnostic) error {
	var errs []error
	for _, d := range diags {
		if d.Severity == tfprotov6.DiagnosticSeverityError {
			errs = append(errs, fmt.Errorf("%s: %s", d.Summary, d.Detail))
		}
	}
	return errors.Join(errs...)
}
