package tfutil

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// stateUnlessChanged implements UseStateForUnknown for computed values the server derives
// from other attributes: when any of those attributes changes, the value is left unknown
// so the plan does not promise a stale value.
type stateUnlessChanged struct {
	deps []path.Expression
}

func (m stateUnlessChanged) Description(context.Context) string {
	return "Keeps the prior value unless an attribute it is derived from changes."
}

func (m stateUnlessChanged) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

// decide reports whether the prior state value should be used.
func (m stateUnlessChanged) decide(ctx context.Context, p path.Path, configVal, stateVal attr.Value, cfg tfsdk.Config, plan tfsdk.Plan, state tfsdk.State, diags *diag.Diagnostics) bool {
	if !configVal.IsNull() || stateVal.IsNull() || state.Raw.IsNull() || plan.Raw.IsNull() {
		return false
	}
	// When the enclosing block is not configured, the block's own plan modifier (see
	// Object) has already decided: an unknown value here means it was reset on purpose.
	if parent := p.ParentPath(); len(parent.Steps()) > 0 {
		var pc attr.Value
		diags.Append(cfg.GetAttribute(ctx, parent, &pc)...)
		if pc == nil || pc.IsNull() {
			return false
		}
	}
	for _, expr := range m.deps {
		paths, d := plan.PathMatches(ctx, p.Expression().Merge(expr))
		diags.Append(d...)
		for _, dp := range paths {
			var pv, sv, cv attr.Value
			diags.Append(plan.GetAttribute(ctx, dp, &pv)...)
			diags.Append(state.GetAttribute(ctx, dp, &sv)...)
			diags.Append(cfg.GetAttribute(ctx, dp, &cv)...)
			if pv == nil || sv == nil {
				return false
			}
			if pv.IsUnknown() {
				// Sibling plan modifiers run in no fixed order: an unset computed
				// dependency may not have received its prior value yet.
				if cv == nil || cv.IsNull() {
					continue
				}
				return false
			}
			if !pv.Equal(sv) {
				return false
			}
		}
	}
	return true
}

// SetStateUnlessChanged keeps the prior value of a computed set unless one of deps
// (expressions relative to the attribute) changes.
func SetStateUnlessChanged(deps ...path.Expression) planmodifier.Set {
	return setStateUnlessChanged{stateUnlessChanged{deps}}
}

type setStateUnlessChanged struct{ stateUnlessChanged }

func (m setStateUnlessChanged) PlanModifySet(ctx context.Context, req planmodifier.SetRequest, resp *planmodifier.SetResponse) {
	if !req.PlanValue.IsUnknown() {
		return
	}
	if m.decide(ctx, req.Path, req.ConfigValue, req.StateValue, req.Config, req.Plan, req.State, &resp.Diagnostics) {
		resp.PlanValue = req.StateValue
	}
}

// StringStateUnlessChanged keeps the prior value of a computed string unless one of deps
// changes.
func StringStateUnlessChanged(deps ...path.Expression) planmodifier.String {
	return stringStateUnlessChanged{stateUnlessChanged{deps}}
}

type stringStateUnlessChanged struct{ stateUnlessChanged }

func (m stringStateUnlessChanged) PlanModifyString(ctx context.Context, req planmodifier.StringRequest, resp *planmodifier.StringResponse) {
	if !req.PlanValue.IsUnknown() {
		return
	}
	if m.decide(ctx, req.Path, req.ConfigValue, req.StateValue, req.Config, req.Plan, req.State, &resp.Diagnostics) {
		resp.PlanValue = req.StateValue
	}
}

// StringFunc is a plan modifier built from a function.
type StringFunc struct {
	Desc string
	Fn   func(ctx context.Context, req planmodifier.StringRequest, resp *planmodifier.StringResponse)
}

func (m StringFunc) Description(context.Context) string         { return m.Desc }
func (m StringFunc) MarkdownDescription(context.Context) string { return m.Desc }
func (m StringFunc) PlanModifyString(ctx context.Context, req planmodifier.StringRequest, resp *planmodifier.StringResponse) {
	m.Fn(ctx, req, resp)
}

// ConfigString reads a string from the configuration at p (null when unset).
func ConfigString(ctx context.Context, cfg tfsdk.Config, p path.Path, diags *diag.Diagnostics) types.String {
	var v types.String
	diags.Append(cfg.GetAttribute(ctx, p, &v)...)
	return v
}

// ConfigIsNull reports whether the configuration value at p is null.
func ConfigIsNull(ctx context.Context, cfg tfsdk.Config, p path.Path, diags *diag.Diagnostics) bool {
	var v attr.Value
	diags.Append(cfg.GetAttribute(ctx, p, &v)...)
	return v == nil || v.IsNull()
}

// BoolStateUnlessChanged keeps the prior value of a computed bool unless one of deps
// changes.
func BoolStateUnlessChanged(deps ...path.Expression) planmodifier.Bool {
	return boolStateUnlessChanged{stateUnlessChanged{deps}}
}

type boolStateUnlessChanged struct{ stateUnlessChanged }

func (m boolStateUnlessChanged) PlanModifyBool(ctx context.Context, req planmodifier.BoolRequest, resp *planmodifier.BoolResponse) {
	if !req.PlanValue.IsUnknown() {
		return
	}
	if m.decide(ctx, req.Path, req.ConfigValue, req.StateValue, req.Config, req.Plan, req.State, &resp.Diagnostics) {
		resp.PlanValue = req.StateValue
	}
}

// MapStateUnlessChanged keeps the prior value of a computed map unless one of deps changes.
func MapStateUnlessChanged(deps ...path.Expression) planmodifier.Map {
	return mapStateUnlessChanged{stateUnlessChanged{deps}}
}

type mapStateUnlessChanged struct{ stateUnlessChanged }

func (m mapStateUnlessChanged) PlanModifyMap(ctx context.Context, req planmodifier.MapRequest, resp *planmodifier.MapResponse) {
	if !req.PlanValue.IsUnknown() {
		return
	}
	if m.decide(ctx, req.Path, req.ConfigValue, req.StateValue, req.Config, req.Plan, req.State, &resp.Diagnostics) {
		resp.PlanValue = req.StateValue
	}
}
