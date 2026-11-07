// Package config implements tenant-wide server configuration singletons (session,
// CORS, inbound protocol settings, DCR, impersonation, remote logging, ...).
package config

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/hayone1/terraform-provider-wso2is/internal/client"
	"github.com/hayone1/terraform-provider-wso2is/internal/tfutil"
)

type kind int

const (
	kindBool kind = iota
	kindInt
	kindString
	kindStringList
	kindStringSet
	kindSecret
)

// list update strategies for JSON-patch styles
const (
	listWhole   = ""        // REPLACE with the whole array
	listIndexed = "indexed" // REMOVE /key/{i} for the old values, ADD /key/{i} for the new
	listAddEach = "add"     // REPLACE /key "" to clear, then ADD /key <value> per value
)

// field maps one attribute to the configuration document.
type field struct {
	attr    string
	kind    kind
	key     string // dotted path in the GET document; also the JSON-patch path (with '/')
	desc    string
	listOps string
}

type style int

const (
	// patchOps sends JSON-patch operations with string values.
	patchOps style = iota
	// patchOpsTyped sends JSON-patch operations with typed (bool/number) values.
	patchOpsTyped
	// patchObject sends the changed fields as a (nested) object.
	patchObject
	// putObject sends the full document with PUT.
	putObject
)

// singleton describes a configuration endpoint.
type singleton struct {
	typeName string
	desc     string
	path     string // relative to /api/server/v1
	style    style
	fields   []field
	// revertPath is called with DELETE on destroy to restore the defaults ("" = keep).
	revertPath string
	// resetBody is sent with PUT on destroy when the API has no revert endpoint but
	// leaving the settings behind would affect unrelated resources.
	resetBody map[string]any
	// extraBody is merged into PUT bodies (fixed fields the API needs).
	extraBody map[string]any
}

type singletonResource struct {
	def    singleton
	client *client.Client
}

func (s singleton) factory() func() resource.Resource {
	return func() resource.Resource { return &singletonResource{def: s} }
}

var (
	_ resource.ResourceWithConfigure   = (*singletonResource)(nil)
	_ resource.ResourceWithImportState = (*singletonResource)(nil)
)

func (r *singletonResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_" + r.def.typeName
}

func (r *singletonResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	attrs := map[string]schema.Attribute{"id": tfutil.IDAttribute("Always `" + r.def.typeName + "`.")}
	for _, f := range r.def.fields {
		switch f.kind {
		case kindBool:
			attrs[f.attr] = tfutil.OCBool(f.desc)
		case kindInt:
			attrs[f.attr] = tfutil.OCInt64(f.desc)
		case kindString:
			attrs[f.attr] = tfutil.OCString(f.desc)
		case kindStringList:
			attrs[f.attr] = tfutil.OCStringList(f.desc)
		case kindStringSet:
			attrs[f.attr] = tfutil.OCStringSet(f.desc)
		case kindSecret:
			attrs[f.attr] = schema.StringAttribute{MarkdownDescription: f.desc + " The server never returns this value.", Optional: true, Sensitive: true}
		}
	}
	restore := "When you destroy this resource, the settings keep their current values, and Terraform stops managing them."
	if r.def.revertPath != "" || r.def.resetBody != nil {
		restore = "When you destroy this resource, the provider restores the server defaults."
	}
	resp.Schema = schema.Schema{
		MarkdownDescription: r.def.desc + " The resource only changes the settings that you set. Other settings keep their current values. " + restore +
			" Each tenant (or organization) has one set of these settings. To import them, use the ID `" + r.def.typeName + "`.",
		Attributes: attrs,
	}
}

func (r *singletonResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = tfutil.ConfigureResource(req, resp)
}

// value reads an attribute from a plan, state or config.
func value(ctx context.Context, src interface {
	GetAttribute(context.Context, path.Path, any) diag.Diagnostics
}, f field, diags *diag.Diagnostics) attr.Value {
	p := path.Root(f.attr)
	switch f.kind {
	case kindBool:
		var v types.Bool
		diags.Append(src.GetAttribute(ctx, p, &v)...)
		return v
	case kindInt:
		var v types.Int64
		diags.Append(src.GetAttribute(ctx, p, &v)...)
		return v
	case kindStringList:
		var v types.List
		diags.Append(src.GetAttribute(ctx, p, &v)...)
		return v
	case kindStringSet:
		var v types.Set
		diags.Append(src.GetAttribute(ctx, p, &v)...)
		return v
	default:
		var v types.String
		diags.Append(src.GetAttribute(ctx, p, &v)...)
		return v
	}
}

// jsonValue converts an attribute value to JSON; asString renders scalars as strings.
func jsonValue(ctx context.Context, v attr.Value, asString bool, diags *diag.Diagnostics) any {
	switch t := v.(type) {
	case types.Bool:
		if asString {
			return strconv.FormatBool(t.ValueBool())
		}
		return t.ValueBool()
	case types.Int64:
		if asString {
			return strconv.FormatInt(t.ValueInt64(), 10)
		}
		return t.ValueInt64()
	case types.List:
		return nonNil(tfutil.Strings(ctx, t, diags))
	case types.Set:
		return nonNil(tfutil.Strings(ctx, t, diags))
	case types.String:
		return t.ValueString()
	}
	return nil
}

func (r *singletonResource) apply(ctx context.Context, plan tfsdk.Plan, state *tfsdk.State, diags *diag.Diagnostics) {
	var changed []field
	values := map[string]attr.Value{}
	for _, f := range r.def.fields {
		pv := value(ctx, plan, f, diags)
		values[f.attr] = pv
		if pv.IsNull() || pv.IsUnknown() {
			continue
		}
		if state != nil && !state.Raw.IsNull() {
			if sv := value(ctx, state, f, diags); sv.Equal(pv) {
				continue
			}
		}
		changed = append(changed, f)
	}
	if diags.HasError() || len(changed) == 0 && r.def.style != putObject {
		return
	}
	p := r.client.ServerPath(r.def.path)
	unlock := r.client.LockTenantConfig()
	defer unlock()
	var err error
	switch r.def.style {
	case patchOps, patchOpsTyped:
		ops := []map[string]any{}
		for _, f := range changed {
			key := "/" + strings.ReplaceAll(f.key, ".", "/")
			isList := f.kind == kindStringList || f.kind == kindStringSet
			switch {
			case isList && f.listOps == listIndexed:
				// Remove what the server has now (state is empty on create).
				var cur map[string]any
				if gerr := r.client.Get(ctx, p, nil, &cur); gerr != nil {
					err = gerr
					break
				}
				raw, _ := client.LookupSCIM(cur, f.key)
				arr, _ := raw.([]any)
				old := make([]string, 0, len(arr))
				for _, e := range arr {
					if s, ok := e.(string); ok {
						old = append(old, s)
					}
				}
				for i := len(old) - 1; i >= 0; i-- {
					ops = append(ops, map[string]any{"operation": "REMOVE", "path": fmt.Sprintf("%s/%d", key, i)})
				}
				for i, v := range jsonValue(ctx, values[f.attr], false, diags).([]string) {
					ops = append(ops, map[string]any{"operation": "ADD", "path": fmt.Sprintf("%s/%d", key, i), "value": v})
				}
			case isList && f.listOps == listAddEach:
				// REPLACE takes one value (or "" to clear); ADD appends one more.
				vals := jsonValue(ctx, values[f.attr], false, diags).([]string)
				if len(vals) == 0 {
					ops = append(ops, map[string]any{"operation": "REPLACE", "path": key, "value": ""})
				}
				for i, v := range vals {
					op := "ADD"
					if i == 0 {
						op = "REPLACE"
					}
					ops = append(ops, map[string]any{"operation": op, "path": key, "value": v})
				}
			default:
				ops = append(ops, map[string]any{"operation": "REPLACE", "path": key, "value": jsonValue(ctx, values[f.attr], r.def.style == patchOps && !isList, diags)})
			}
		}
		if err == nil {
			_, err = r.client.Patch(ctx, p, ops, nil)
		}
	case patchObject:
		body := map[string]any{}
		for _, f := range changed {
			client.SetSCIM(body, f.key, jsonValue(ctx, values[f.attr], false, diags))
		}
		_, err = r.client.Patch(ctx, p, body, nil)
	case putObject:
		var cur map[string]any
		if gerr := r.client.Get(ctx, p, nil, &cur); gerr != nil && !client.IsNotFound(gerr) {
			err = gerr
			break
		}
		if cur == nil {
			cur = map[string]any{}
		}
		for k, v := range r.def.extraBody {
			cur[k] = v
		}
		for _, f := range r.def.fields {
			v := values[f.attr]
			if v.IsNull() || v.IsUnknown() {
				continue
			}
			client.SetSCIM(cur, f.key, jsonValue(ctx, v, false, diags))
		}
		_, err = r.client.Put(ctx, p, cur, nil)
	}
	if err != nil {
		tfutil.AddAPIError(diags, "update "+strings.ReplaceAll(r.def.typeName, "_", " "), err)
	}
}

func (r *singletonResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	r.apply(ctx, req.Plan, nil, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	r.read(ctx, req.Plan, &resp.State, &resp.Diagnostics)
}

func (r *singletonResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	r.apply(ctx, req.Plan, &req.State, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	r.read(ctx, req.Plan, &resp.State, &resp.Diagnostics)
}

func (r *singletonResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	src := tfsdk.Plan{Schema: req.State.Schema, Raw: req.State.Raw}
	r.read(ctx, src, &resp.State, &resp.Diagnostics)
}

func (r *singletonResource) read(ctx context.Context, prior tfsdk.Plan, out *tfsdk.State, diags *diag.Diagnostics) {
	// Reads take the lock too: a read during another singleton's update of the resident
	// identity provider can fail (CNF-65011) or see half-written settings.
	unlock := r.client.LockTenantConfig()
	var doc map[string]any
	err := r.client.Get(ctx, r.client.ServerPath(r.def.path), nil, &doc)
	unlock()
	if err != nil && !client.IsNotFound(err) {
		tfutil.AddAPIError(diags, "read "+strings.ReplaceAll(r.def.typeName, "_", " "), err)
		return
	}
	diags.Append(out.SetAttribute(ctx, path.Root("id"), r.def.typeName)...)
	for _, f := range r.def.fields {
		raw, _ := client.LookupSCIM(doc, f.key)
		var v attr.Value
		switch f.kind {
		case kindBool:
			switch t := raw.(type) {
			case bool:
				v = types.BoolValue(t)
			case string:
				v = types.BoolValue(t == "true")
			default:
				v = types.BoolValue(false)
			}
		case kindInt:
			switch t := raw.(type) {
			case float64:
				v = types.Int64Value(int64(math.Round(t)))
			case string:
				i, err := strconv.ParseInt(t, 10, 64)
				if err != nil {
					v = types.Int64Null()
				} else {
					v = types.Int64Value(i)
				}
			default:
				v = types.Int64Null()
			}
		case kindStringList:
			var list []string
			if arr, ok := raw.([]any); ok {
				for _, e := range arr {
					if s, ok := e.(string); ok {
						list = append(list, s)
					}
				}
			}
			v = tfutil.ListOfStrings(list, false)
		case kindStringSet:
			var list []string
			if arr, ok := raw.([]any); ok {
				for _, e := range arr {
					if s, ok := e.(string); ok {
						list = append(list, s)
					}
				}
			}
			v = tfutil.SetOfStrings(list, false)
		case kindSecret:
			// Write-only: keep the configured value.
			v = value(ctx, prior, f, diags)
		default:
			s, _ := client.SCIMString(raw)
			v = types.StringValue(s)
		}
		diags.Append(out.SetAttribute(ctx, path.Root(f.attr), v)...)
	}
}

func (r *singletonResource) Delete(ctx context.Context, _ resource.DeleteRequest, resp *resource.DeleteResponse) {
	if r.def.revertPath == "" && r.def.resetBody == nil {
		return
	}
	unlock := r.client.LockTenantConfig()
	defer unlock()
	var err error
	if r.def.revertPath != "" {
		err = r.client.Delete(ctx, r.client.ServerPath(r.def.revertPath))
	} else {
		_, err = r.client.Put(ctx, r.client.ServerPath(r.def.path), r.def.resetBody, nil)
	}
	if err != nil && !client.IsNotFound(err) {
		tfutil.AddAPIError(&resp.Diagnostics, fmt.Sprintf("restore the default %s", strings.ReplaceAll(r.def.typeName, "_", " ")), err)
	}
}

func (r *singletonResource) ImportState(ctx context.Context, _ resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), r.def.typeName)...)
}

var _ = planmodifier.String(nil)

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
