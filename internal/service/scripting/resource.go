// Package scripting implements adaptive authentication script libraries and secrets.
package scripting

import (
	"bytes"
	"context"
	"mime/multipart"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/hayone1/terraform-provider-wso2is/internal/client"
	"github.com/hayone1/terraform-provider-wso2is/internal/tfutil"
)

// ---- wso2is_script_library ----

var (
	_ resource.ResourceWithConfigure   = (*libraryResource)(nil)
	_ resource.ResourceWithImportState = (*libraryResource)(nil)
)

// NewLibraryResource returns the wso2is_script_library resource.
func NewLibraryResource() resource.Resource { return &libraryResource{} }

type libraryResource struct{ client *client.Client }

type libraryModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
	Content     types.String `tfsdk:"content"`
}

func (r *libraryResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_script_library"
}

func (r *libraryResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a JavaScript library for adaptive authentication scripts. A script loads the library with `var lib = require('<name>');`. The library shares its functions through `module.exports`. To import, use the library name.",
		Attributes: map[string]schema.Attribute{
			"id": tfutil.IDAttribute("Library name."),
			"name": schema.StringAttribute{
				MarkdownDescription: "The library name. It must end with `.js`. Changing the name replaces the library.",
				Required:            true,
				Validators:          []validator.String{stringvalidator.RegexMatches(mustRegexp(`\.js$`), "must end with .js")},
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"description": schema.StringAttribute{MarkdownDescription: "Description.", Optional: true, Computed: true, Default: stringdefault.StaticString("")},
			"content":     schema.StringAttribute{MarkdownDescription: "JavaScript source (for example `file(\"libs/risk.js\")`).", Required: true},
		},
	}
}

func (r *libraryResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = tfutil.ConfigureResource(req, resp)
}

func multipartBody(fields map[string]string, fileField, fileName, content string) ([]byte, string, error) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	for _, k := range tfutil.SortedKeys(fields) {
		if err := w.WriteField(k, fields[k]); err != nil {
			return nil, "", err
		}
	}
	fw, err := w.CreateFormFile(fileField, fileName)
	if err != nil {
		return nil, "", err
	}
	if _, err := fw.Write([]byte(content)); err != nil {
		return nil, "", err
	}
	if err := w.Close(); err != nil {
		return nil, "", err
	}
	return buf.Bytes(), w.FormDataContentType(), nil
}

func (r *libraryResource) send(ctx context.Context, method string, m *libraryModel, create bool) error {
	fields := map[string]string{"description": m.Description.ValueString()}
	p := r.client.ServerPath("/script-libraries")
	if create {
		fields["name"] = m.Name.ValueString()
	} else {
		p += "/" + url.PathEscape(m.Name.ValueString())
	}
	body, ct, err := multipartBody(fields, "content", m.Name.ValueString(), m.Content.ValueString())
	if err != nil {
		return err
	}
	_, err = r.client.Do(ctx, client.Request{Method: method, Path: p, Body: body, ContentType: ct})
	return err
}

func (r *libraryResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan libraryModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.send(ctx, http.MethodPost, &plan, true); err != nil {
		tfutil.AddAPIError(&resp.Diagnostics, "create script library", err)
		return
	}
	plan.ID = plan.Name
	r.read(ctx, &plan, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *libraryResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state libraryModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !r.read(ctx, &state, &resp.Diagnostics) {
		if !resp.Diagnostics.HasError() {
			resp.State.RemoveResource(ctx)
		}
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *libraryResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan libraryModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.send(ctx, http.MethodPut, &plan, false); err != nil {
		tfutil.AddAPIError(&resp.Diagnostics, "update script library", err)
		return
	}
	plan.ID = plan.Name
	r.read(ctx, &plan, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *libraryResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state libraryModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.Delete(ctx, r.client.ServerPath("/script-libraries/"+url.PathEscape(state.Name.ValueString()))); err != nil && !client.IsNotFound(err) {
		tfutil.AddAPIError(&resp.Diagnostics, "delete script library", err)
	}
}

func (r *libraryResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), req.ID)...)
}

func (r *libraryResource) read(ctx context.Context, m *libraryModel, diags *diag.Diagnostics) bool {
	var lib map[string]any
	base := r.client.ServerPath("/script-libraries/" + url.PathEscape(m.Name.ValueString()))
	if err := r.client.Get(ctx, base, nil, &lib); err != nil {
		if client.IsNotFound(err) {
			return false
		}
		tfutil.AddAPIError(diags, "read script library", err)
		return false
	}
	m.Description = tfutil.GetStringOrEmpty(lib, "description")
	resp, err := r.client.Do(ctx, client.Request{Method: http.MethodGet, Path: base + "/content", Accept: "*/*"})
	if err != nil {
		tfutil.AddAPIError(diags, "read script library content", err)
		return false
	}
	content := string(resp.Body)
	// Keep the configured text when only line endings or trailing whitespace differ.
	if !tfutil.Known(m.Content) || strings.TrimSpace(strings.ReplaceAll(m.Content.ValueString(), "\r\n", "\n")) != strings.TrimSpace(strings.ReplaceAll(content, "\r\n", "\n")) {
		m.Content = types.StringValue(content)
	}
	return true
}

// ---- wso2is_secret ----

var (
	_ resource.ResourceWithConfigure   = (*secretResource)(nil)
	_ resource.ResourceWithImportState = (*secretResource)(nil)
)

// NewSecretResource returns the wso2is_secret resource.
func NewSecretResource() resource.Resource { return &secretResource{} }

type secretResource struct{ client *client.Client }

type secretModel struct {
	ID           types.String `tfsdk:"id"`
	Type         types.String `tfsdk:"type"`
	Name         types.String `tfsdk:"name"`
	Value        types.String `tfsdk:"value"`
	ValueWO      types.String `tfsdk:"value_wo"`
	ValueVersion types.Int64  `tfsdk:"value_wo_version"`
	Description  types.String `tfsdk:"description"`
	SecretID     types.String `tfsdk:"secret_id"`
}

func (r *secretResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_secret"
}

func (r *secretResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Stores a secret for adaptive authentication scripts. Scripts read the secret as `secrets.<name>`, for example in `httpGet` or `httpPost` calls. The server never returns the value, so the provider cannot notice changes made outside Terraform. To import, use the ID `<type>/<name>`.",
		Attributes: map[string]schema.Attribute{
			"id": tfutil.IDAttribute("`<type>/<name>`."),
			"type": schema.StringAttribute{
				MarkdownDescription: "Secret type. The default is `ADAPTIVE_AUTH_CALL_CHOREO`, the type adaptive scripts read.",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("ADAPTIVE_AUTH_CALL_CHOREO"),
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Secret name (letters, digits and underscores). Changing it replaces the secret.",
				Required:            true,
				Validators:          []validator.String{stringvalidator.RegexMatches(mustRegexp(`^[A-Za-z0-9_]+$`), "only letters, digits and underscores")},
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"value":            schema.StringAttribute{MarkdownDescription: "The secret value. Terraform stores it in the state. Use `value_wo` instead to keep it out of the state.", Optional: true, Sensitive: true},
			"value_wo":         schema.StringAttribute{MarkdownDescription: "Write-only secret value (Terraform 1.11+). Change `value_wo_version` to update it.", Optional: true, Sensitive: true, WriteOnly: true},
			"value_wo_version": schema.Int64Attribute{MarkdownDescription: "Increase this number to send a new `value_wo`.", Optional: true},
			"description":      schema.StringAttribute{MarkdownDescription: "Description.", Optional: true, Computed: true, Default: stringdefault.StaticString("")},
			"secret_id":        tfutil.ComputedString("Server-side secret ID."),
		},
	}
}

func (r *secretResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = tfutil.ConfigureResource(req, resp)
}

func (r *secretResource) path(m *secretModel) string {
	return r.client.ServerPath("/secrets/" + url.PathEscape(m.Type.ValueString()) + "/" + url.PathEscape(m.Name.ValueString()))
}

func (r *secretResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan, cfg secretModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	value := plan.Value.ValueString()
	if value == "" {
		value = cfg.ValueWO.ValueString()
	}
	if value == "" {
		resp.Diagnostics.AddError("Missing secret value", "Set value or value_wo.")
		return
	}
	body := map[string]string{"name": plan.Name.ValueString(), "value": value, "description": plan.Description.ValueString()}
	if _, err := r.client.Post(ctx, r.client.ServerPath("/secrets/"+url.PathEscape(plan.Type.ValueString())), body, nil); err != nil {
		tfutil.AddAPIError(&resp.Diagnostics, "create secret", err)
		return
	}
	plan.ID = types.StringValue(plan.Type.ValueString() + "/" + plan.Name.ValueString())
	r.read(ctx, &plan, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *secretResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state secretModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !r.read(ctx, &state, &resp.Diagnostics) {
		if !resp.Diagnostics.HasError() {
			resp.State.RemoveResource(ctx)
		}
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *secretResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state, cfg secretModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	value := plan.Value.ValueString()
	if value == "" {
		value = cfg.ValueWO.ValueString()
	}
	body := map[string]string{"value": value, "description": plan.Description.ValueString()}
	if value == "" {
		// Only the description changed: the API needs the value, so patch the description.
		if _, err := r.client.Patch(ctx, r.path(&plan), []map[string]string{{"operation": "REPLACE", "path": "/description", "value": plan.Description.ValueString()}}, nil); err != nil {
			tfutil.AddAPIError(&resp.Diagnostics, "update secret", err)
			return
		}
	} else if _, err := r.client.Put(ctx, r.path(&plan), body, nil); err != nil {
		tfutil.AddAPIError(&resp.Diagnostics, "update secret", err)
		return
	}
	plan.ID = state.ID
	r.read(ctx, &plan, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *secretResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state secretModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.Delete(ctx, r.path(&state)); err != nil && !client.IsNotFound(err) {
		tfutil.AddAPIError(&resp.Diagnostics, "delete secret", err)
	}
}

func (r *secretResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	typ, name, ok := strings.Cut(req.ID, "/")
	if !ok {
		typ, name = "ADAPTIVE_AUTH_CALL_CHOREO", req.ID
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), typ+"/"+name)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("type"), typ)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), name)...)
}

func (r *secretResource) read(ctx context.Context, m *secretModel, diags *diag.Diagnostics) bool {
	var s map[string]any
	if err := r.client.Get(ctx, r.path(m), nil, &s); err != nil {
		if client.IsNotFound(err) || client.HasCode(err, "SECRETM-60002") {
			return false
		}
		tfutil.AddAPIError(diags, "read secret", err)
		return false
	}
	m.SecretID = tfutil.GetStringOrEmpty(s, "secretId")
	m.Description = tfutil.GetStringOrEmpty(s, "description")
	return true
}

func mustRegexp(s string) *regexp.Regexp { return regexp.MustCompile(s) }
