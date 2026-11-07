// Package application implements wso2is_application and related resources.
package application

import (
	"context"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/hayone1/terraform-provider-wso2is/internal/client"
	"github.com/hayone1/terraform-provider-wso2is/internal/tfutil"
)

var (
	_ resource.Resource                = (*applicationResource)(nil)
	_ resource.ResourceWithConfigure   = (*applicationResource)(nil)
	_ resource.ResourceWithImportState = (*applicationResource)(nil)
)

// NewResource returns the wso2is_application resource.
func NewResource() resource.Resource { return &applicationResource{} }

type applicationResource struct {
	client *client.Client
}

func (r *applicationResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_application"
}

func (r *applicationResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceSchema()
}

func (r *applicationResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = tfutil.ConfigureResource(req, resp)
}

func (r *applicationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan appModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	app := expandApplication(ctx, &plan, &resp.Diagnostics)
	inbound := &client.InboundProtocols{}
	if o, ok := tfutil.ObjectAs[oidcModel](ctx, plan.OIDC, &resp.Diagnostics); ok {
		// The server dereferences nested settings without null checks (a partial
		// access_token fails with HTTP 500), so create with the basics and apply the
		// rest on top of the server defaults below.
		inbound.OIDC = minimalOIDC(expandOIDC(ctx, o, &resp.Diagnostics))
	}
	if s, ok := tfutil.ObjectAs[samlModel](ctx, plan.SAML, &resp.Diagnostics); ok {
		inbound.SAML = minimalSAML(expandSAML(ctx, s, &resp.Diagnostics))
	}
	if p, ok := tfutil.ObjectAs[passiveSTSModel](ctx, plan.PassiveSTS, &resp.Diagnostics); ok {
		inbound.PassiveSTS = &client.PassiveSTSConfig{Realm: p.Realm.ValueString(), ReplyTo: p.ReplyTo.ValueString(), ReplyToLogout: p.ReplyToLogout.ValueString()}
	}
	if w, ok := tfutil.ObjectAs[wsTrustModel](ctx, plan.WSTrust, &resp.Diagnostics); ok {
		inbound.WSTrust = &client.WSTrustConfig{Audience: w.Audience.ValueString(), CertificateAlias: w.CertificateAlias.ValueString()}
	}
	if inbound.OIDC != nil || inbound.SAML != nil || inbound.PassiveSTS != nil || inbound.WSTrust != nil {
		app.InboundProtocolConfiguration = inbound
	}
	if resp.Diagnostics.HasError() {
		return
	}

	id, err := r.client.CreateApplication(ctx, app)
	if err != nil {
		tfutil.AddAPIError(&resp.Diagnostics, "create application", err)
		return
	}
	// Save the ID right away so a failure below does not orphan the application.
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), id)...)

	// Apply the protocol settings left out of the create request.
	if !plan.OIDC.IsNull() || !plan.SAML.IsNull() {
		r.applyInboundAfterCreate(ctx, id, &plan, &resp.Diagnostics)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	state := plan
	state.ID = types.StringValue(id)
	if found := r.read(ctx, &state, &plan, &resp.Diagnostics); !found && !resp.Diagnostics.HasError() {
		resp.Diagnostics.AddError("Application not found after creation", "The server accepted the application but it could not be read back. ID: "+id)
	}
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *applicationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state appModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	prior := state
	if !r.read(ctx, &state, &prior, &resp.Diagnostics) {
		if !resp.Diagnostics.HasError() {
			resp.State.RemoveResource(ctx)
		}
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *applicationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state appModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	id := state.ID.ValueString()

	patch := r.expandPatch(ctx, req, &plan, &state, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	if patch != nil {
		if err := r.client.PatchApplication(ctx, id, patch); err != nil {
			tfutil.AddAPIError(&resp.Diagnostics, "update application", err)
			return
		}
	}

	// PATCH ignores an empty script; the script endpoint clears it.
	if ps, ss := planScript(ctx, plan.AuthenticationSequence, &resp.Diagnostics), planScript(ctx, state.AuthenticationSequence, &resp.Diagnostics); ps != nil && *ps == "" && ss != nil && *ss != "" {
		if err := r.client.PutAuthenticationScript(ctx, id, ""); err != nil {
			tfutil.AddAPIError(&resp.Diagnostics, "remove the adaptive authentication script", err)
			return
		}
	}

	r.updateInbound(ctx, id, &plan, &state, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	plan.ID = state.ID
	newState := plan
	if found := r.read(ctx, &newState, &plan, &resp.Diagnostics); !found && !resp.Diagnostics.HasError() {
		resp.Diagnostics.AddError("Application not found after update", "ID: "+id)
	}
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

// applyInboundAfterCreate merges the full planned OIDC/SAML settings over what the
// server created, when they go beyond the minimal create request.
func (r *applicationResource) applyInboundAfterCreate(ctx context.Context, id string, plan *appModel, diags *diag.Diagnostics) {
	if o, ok := tfutil.ObjectAs[oidcModel](ctx, plan.OIDC, diags); ok {
		full := client.PruneEmpty(expandOIDC(ctx, o, diags).Map(), nil)
		if len(full) > len(minimalOIDC(full)) {
			cur, err := r.client.GetInboundOIDC(ctx, id)
			if err != nil {
				tfutil.AddAPIError(diags, "read OpenID Connect configuration", err)
				return
			}
			delete(cur, "clientSecret")
			delete(cur, "state")
			delete(full, "clientSecret")
			known := client.CloneJSON(cur)
			if err := r.client.PutInboundOIDC(ctx, id, compatOIDC(client.MergeJSON(cur, full), known)); err != nil {
				tfutil.AddAPIError(diags, "configure OpenID Connect", err)
				return
			}
		}
	}
	if s, ok := tfutil.ObjectAs[samlModel](ctx, plan.SAML, diags); ok {
		body := expandSAML(ctx, s, diags)
		if body.ManualConfiguration != nil {
			full := client.PruneEmpty(body.ManualConfiguration, nil)
			if len(full) > len(minimalSAML(body).ManualConfiguration) {
				cur, err := r.client.GetInboundSAML(ctx, id)
				if err != nil {
					tfutil.AddAPIError(diags, "read SAML configuration", err)
					return
				}
				if err := r.client.PutInboundSAML(ctx, id, &client.SAMLInbound{ManualConfiguration: client.MergeJSON(cur, full)}); err != nil {
					tfutil.AddAPIError(diags, "configure SAML", err)
					return
				}
			}
		}
	}
}

// compatOIDC removes settings that servers older than 7.3 reject as unknown fields
// (HTTP 400), when the server did not return them (cur) and they hold the default.
func compatOIDC(body, cur map[string]any) map[string]any {
	if _, ok := cur["cibaAuthenticationRequest"]; !ok {
		delete(body, "cibaAuthenticationRequest")
	}
	at, _ := body["accessToken"].(map[string]any)
	curAT, _ := cur["accessToken"].(map[string]any)
	if v, ok := at["enableJwtScopeAsArray"].(bool); ok && !v {
		if _, known := curAT["enableJwtScopeAsArray"]; !known {
			delete(at, "enableJwtScopeAsArray")
		}
	}
	return body
}

// minimalOIDC keeps the settings that are safe (and, for the credentials and FAPI flag,
// required) in the create request.
func minimalOIDC(full map[string]any) map[string]any {
	out := map[string]any{}
	for _, k := range []string{"clientId", "clientSecret", "grantTypes", "callbackURLs", "allowedOrigins", "publicClient", "isFAPIApplication", "issuer"} {
		if v, ok := full[k]; ok && v != "" {
			out[k] = v
		}
	}
	return out
}

// minimalSAML keeps the identity of the service provider for the create request.
func minimalSAML(full *client.SAMLInbound) *client.SAMLInbound {
	if full.ManualConfiguration == nil {
		return full
	}
	out := map[string]any{}
	for _, k := range []string{"issuer", "serviceProviderQualifier", "assertionConsumerUrls", "defaultAssertionConsumerUrl"} {
		if v, ok := full.ManualConfiguration[k]; ok && v != "" {
			out[k] = v
		}
	}
	return &client.SAMLInbound{ManualConfiguration: out}
}

// expandPatch builds the PATCH body. The server replaces every nested section it receives
// as a whole (a partial claimConfiguration drops the requested claims), so a section is
// sent complete when anything in it changed, and left out otherwise. It returns nil when
// nothing outside the inbound protocols changed.
func (r *applicationResource) expandPatch(ctx context.Context, req resource.UpdateRequest, plan, state *appModel, diags *diag.Diagnostics) *client.Application {
	full := expandApplication(ctx, plan, diags)
	patch := &client.Application{}
	changed := false
	setStr := func(dst **string, src *string, p, s types.String) {
		if !p.Equal(s) {
			*dst, changed = src, true
		}
	}
	if !plan.Name.Equal(state.Name) {
		patch.Name, changed = full.Name, true
	}
	setStr(&patch.Description, full.Description, plan.Description, state.Description)
	setStr(&patch.ImageURL, full.ImageURL, plan.ImageURL, state.ImageURL)
	setStr(&patch.AccessURL, full.AccessURL, plan.AccessURL, state.AccessURL)
	setStr(&patch.LogoutReturnURL, full.LogoutReturnURL, plan.LogoutReturnURL, state.LogoutReturnURL)
	setStr(&patch.TemplateID, full.TemplateID, plan.TemplateID, state.TemplateID)
	setStr(&patch.TemplateVersion, full.TemplateVersion, plan.TemplateVersion, state.TemplateVersion)
	if !plan.Enabled.Equal(state.Enabled) {
		patch.ApplicationEnabled, changed = full.ApplicationEnabled, true
	}
	if !plan.EnhancedOrgAuthenticationEnabled.Equal(state.EnhancedOrgAuthenticationEnabled) {
		patch.EnhancedOrgAuthenticationEnabled, changed = full.EnhancedOrgAuthenticationEnabled, true
	}
	if !plan.AssociatedRoles.Equal(state.AssociatedRoles) {
		patch.AssociatedRoles, changed = full.AssociatedRoles, true
	}
	if !plan.ClaimConfiguration.Equal(state.ClaimConfiguration) && full.ClaimConfiguration != nil {
		patch.ClaimConfiguration, changed = full.ClaimConfiguration, true
		// The server reports http://wso2.org/claims/role as the default role claim but
		// rejects it in updates (the 7.x claim is .../roles): only send a configured value.
		var roleClaim types.String
		diags.Append(req.Config.GetAttribute(ctx, path.Root("claim_configuration").AtName("role").AtName("claim"), &roleClaim)...)
		if patch.ClaimConfiguration.Role != nil && roleClaim.IsNull() {
			patch.ClaimConfiguration.Role.Claim = nil
		}
	}
	if !plan.AuthenticationSequence.Equal(state.AuthenticationSequence) {
		patch.AuthenticationSequence, changed = full.AuthenticationSequence, true
	}
	if !plan.AdvancedConfiguration.Equal(state.AdvancedConfiguration) {
		patch.AdvancedConfigurations, changed = full.AdvancedConfigurations, true
	}
	if !plan.Provisioning.Equal(state.Provisioning) {
		patch.ProvisioningConfigurations, changed = full.ProvisioningConfigurations, true
	}
	if !changed {
		return nil
	}
	return patch
}

func (r *applicationResource) updateInbound(ctx context.Context, id string, plan, state *appModel, diags *diag.Diagnostics) {
	// OpenID Connect: read-modify-write so unmanaged fields survive. The server rejects
	// a client secret in updates, and needs the client ID.
	if o, ok := tfutil.ObjectAs[oidcModel](ctx, plan.OIDC, diags); ok {
		body := map[string]any{}
		if !state.OIDC.IsNull() {
			cur, err := r.client.GetInboundOIDC(ctx, id)
			if err != nil && !client.IsNotFound(err) {
				tfutil.AddAPIError(diags, "read OpenID Connect configuration", err)
				return
			}
			body = cur
			delete(body, "clientSecret")
			delete(body, "state")
		}
		overlay := expandOIDC(ctx, o, diags).Map()
		if so, ok := tfutil.ObjectAs[oidcModel](ctx, state.OIDC, diags); ok {
			// Send only what changed; the rest is the server's current configuration.
			overlay = client.DiffJSON(overlay, expandOIDC(ctx, so, diags).Map())
			delete(overlay, "clientSecret")
		} else {
			overlay = client.PruneEmpty(overlay, nil)
		}
		known := client.CloneJSON(body)
		body = compatOIDC(client.MergeJSON(body, overlay), known)
		if err := r.client.PutInboundOIDC(ctx, id, body); err != nil {
			tfutil.AddAPIError(diags, "update OpenID Connect configuration", err)
			return
		}
	} else if plan.OIDC.IsNull() && !state.OIDC.IsNull() {
		if err := r.client.DeleteInboundProtocol(ctx, id, "oidc"); err != nil && !client.IsNotFound(err) {
			tfutil.AddAPIError(diags, "remove OpenID Connect from the application", err)
			return
		}
	}

	if s, ok := tfutil.ObjectAs[samlModel](ctx, plan.SAML, diags); ok {
		body := expandSAML(ctx, s, diags)
		if ss, ok := tfutil.ObjectAs[samlModel](ctx, state.SAML, diags); ok && body.ManualConfiguration != nil {
			cur, err := r.client.GetInboundSAML(ctx, id)
			if err != nil && !client.IsNotFound(err) {
				tfutil.AddAPIError(diags, "read SAML configuration", err)
				return
			}
			prev := expandSAML(ctx, ss, diags).ManualConfiguration
			body.ManualConfiguration = client.MergeJSON(cur, client.DiffJSON(body.ManualConfiguration, prev))
		} else if body.ManualConfiguration != nil {
			body.ManualConfiguration = client.PruneEmpty(body.ManualConfiguration, nil)
		}
		if err := r.client.PutInboundSAML(ctx, id, body); err != nil {
			tfutil.AddAPIError(diags, "update SAML configuration", err)
			return
		}
	} else if plan.SAML.IsNull() && !state.SAML.IsNull() {
		if err := r.client.DeleteInboundProtocol(ctx, id, "saml"); err != nil && !client.IsNotFound(err) {
			tfutil.AddAPIError(diags, "remove SAML from the application", err)
			return
		}
	}

	if p, ok := tfutil.ObjectAs[passiveSTSModel](ctx, plan.PassiveSTS, diags); ok {
		cfg := &client.PassiveSTSConfig{Realm: p.Realm.ValueString(), ReplyTo: p.ReplyTo.ValueString(), ReplyToLogout: p.ReplyToLogout.ValueString()}
		if err := r.client.PutInboundPassiveSTS(ctx, id, cfg); err != nil {
			tfutil.AddAPIError(diags, "update WS-Federation configuration", err)
			return
		}
	} else if plan.PassiveSTS.IsNull() && !state.PassiveSTS.IsNull() {
		if err := r.client.DeleteInboundProtocol(ctx, id, "passive-sts"); err != nil && !client.IsNotFound(err) {
			tfutil.AddAPIError(diags, "remove WS-Federation from the application", err)
			return
		}
	}

	if w, ok := tfutil.ObjectAs[wsTrustModel](ctx, plan.WSTrust, diags); ok {
		cfg := &client.WSTrustConfig{Audience: w.Audience.ValueString(), CertificateAlias: w.CertificateAlias.ValueString()}
		if err := r.client.PutInboundWSTrust(ctx, id, cfg); err != nil {
			tfutil.AddAPIError(diags, "update WS-Trust configuration", err)
			return
		}
	} else if plan.WSTrust.IsNull() && !state.WSTrust.IsNull() {
		if err := r.client.DeleteInboundProtocol(ctx, id, "ws-trust"); err != nil && !client.IsNotFound(err) {
			tfutil.AddAPIError(diags, "remove WS-Trust from the application", err)
			return
		}
	}
}

func (r *applicationResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state appModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteApplication(ctx, state.ID.ValueString()); err != nil && !client.IsNotFound(err) {
		tfutil.AddAPIError(&resp.Diagnostics, "delete application", err)
	}
}

func (r *applicationResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// read refreshes m from the server. prior carries configured values the server does not
// return verbatim. It returns false when the application no longer exists.
func (r *applicationResource) read(ctx context.Context, m, prior *appModel, diags *diag.Diagnostics) bool {
	app, err := r.client.GetApplication(ctx, m.ID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			return false
		}
		tfutil.AddAPIError(diags, "read application", err)
		return false
	}
	flattenApplication(ctx, app, m, prior, diags)

	protocols := map[string]bool{}
	for _, p := range app.InboundProtocols {
		protocols[p.Type] = true
	}

	m.OIDC = types.ObjectNull(oidcTypes)
	if protocols["oauth2"] {
		o, err := r.client.GetInboundOIDC(ctx, m.ID.ValueString())
		if err != nil {
			tfutil.AddAPIError(diags, "read OpenID Connect configuration", err)
			return false
		}
		po, _ := tfutil.ObjectAs[oidcModel](ctx, prior.OIDC, diags)
		m.OIDC = flattenOIDC(ctx, o, po, diags)
	}

	m.SAML = types.ObjectNull(samlTypes)
	if protocols["samlsso"] {
		s, err := r.client.GetInboundSAML(ctx, m.ID.ValueString())
		if err != nil {
			tfutil.AddAPIError(diags, "read SAML configuration", err)
			return false
		}
		ps, _ := tfutil.ObjectAs[samlModel](ctx, prior.SAML, diags)
		m.SAML = flattenSAML(ctx, s, ps, diags)
	}

	m.PassiveSTS = types.ObjectNull(passiveSTSTypes)
	if protocols["passivests"] {
		p, err := r.client.GetInboundPassiveSTS(ctx, m.ID.ValueString())
		if err != nil {
			tfutil.AddAPIError(diags, "read WS-Federation configuration", err)
			return false
		}
		m.PassiveSTS = flattenPassiveSTS(ctx, p, diags)
	}

	m.WSTrust = types.ObjectNull(wsTrustTypes)
	if protocols["wstrust"] {
		w, err := r.client.GetInboundWSTrust(ctx, m.ID.ValueString())
		if err != nil {
			tfutil.AddAPIError(diags, "read WS-Trust configuration", err)
			return false
		}
		m.WSTrust = flattenWSTrust(ctx, w, diags)
	}
	return !diags.HasError()
}

// changedFromKnown requires replacement when a create-only value is changed explicitly.
func changedFromKnown(_ context.Context, req planmodifier.StringRequest, resp *stringplanmodifier.RequiresReplaceIfFuncResponse) {
	resp.RequiresReplace = tfutil.Known(req.StateValue) && tfutil.Known(req.PlanValue) && req.StateValue.ValueString() != "" && !req.StateValue.Equal(req.PlanValue)
}

func normalizeScript(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = strings.TrimRight(l, " \t")
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func planScript(ctx context.Context, obj types.Object, diags *diag.Diagnostics) *string {
	as, ok := tfutil.ObjectAs[authSeqModel](ctx, obj, diags)
	if !ok || !tfutil.Known(as.Script) {
		return nil
	}
	v := as.Script.ValueString()
	return &v
}
