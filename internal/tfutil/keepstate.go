package tfutil

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
)

// The framework's UseStateForUnknown copies the prior value even when it is null. For an
// attribute inside a nested object that did not exist before (for example adding `saml`
// to an existing application), that plans null for values the server is about to fill in
// and fails with "inconsistent result after apply". These variants only reuse non-null
// prior values.

const keepStateDesc = "Keeps the prior non-null value when no value is configured."

type keepState struct{}

func (keepState) Description(context.Context) string         { return keepStateDesc }
func (keepState) MarkdownDescription(context.Context) string { return keepStateDesc }

func keep(stateIsNull, planIsUnknown, configIsUnknown bool) bool {
	return !stateIsNull && planIsUnknown && !configIsUnknown
}

type keepStateString struct{ keepState }

// KeepStateString is a null-safe UseStateForUnknown for strings.
func KeepStateString() planmodifier.String { return keepStateString{} }

func (keepStateString) PlanModifyString(_ context.Context, req planmodifier.StringRequest, resp *planmodifier.StringResponse) {
	if !req.State.Raw.IsNull() && keep(req.StateValue.IsNull(), req.PlanValue.IsUnknown(), req.ConfigValue.IsUnknown()) {
		resp.PlanValue = req.StateValue
	}
}

type keepStateBool struct{ keepState }

// KeepStateBool is a null-safe UseStateForUnknown for bools.
func KeepStateBool() planmodifier.Bool { return keepStateBool{} }

func (keepStateBool) PlanModifyBool(_ context.Context, req planmodifier.BoolRequest, resp *planmodifier.BoolResponse) {
	if !req.State.Raw.IsNull() && keep(req.StateValue.IsNull(), req.PlanValue.IsUnknown(), req.ConfigValue.IsUnknown()) {
		resp.PlanValue = req.StateValue
	}
}

type keepStateInt64 struct{ keepState }

// KeepStateInt64 is a null-safe UseStateForUnknown for numbers.
func KeepStateInt64() planmodifier.Int64 { return keepStateInt64{} }

func (keepStateInt64) PlanModifyInt64(_ context.Context, req planmodifier.Int64Request, resp *planmodifier.Int64Response) {
	if !req.State.Raw.IsNull() && keep(req.StateValue.IsNull(), req.PlanValue.IsUnknown(), req.ConfigValue.IsUnknown()) {
		resp.PlanValue = req.StateValue
	}
}

type keepStateList struct{ keepState }

// KeepStateList is a null-safe UseStateForUnknown for lists.
func KeepStateList() planmodifier.List { return keepStateList{} }

func (keepStateList) PlanModifyList(_ context.Context, req planmodifier.ListRequest, resp *planmodifier.ListResponse) {
	if !req.State.Raw.IsNull() && keep(req.StateValue.IsNull(), req.PlanValue.IsUnknown(), req.ConfigValue.IsUnknown()) {
		resp.PlanValue = req.StateValue
	}
}

type keepStateSet struct{ keepState }

// KeepStateSet is a null-safe UseStateForUnknown for sets.
func KeepStateSet() planmodifier.Set { return keepStateSet{} }

func (keepStateSet) PlanModifySet(_ context.Context, req planmodifier.SetRequest, resp *planmodifier.SetResponse) {
	if !req.State.Raw.IsNull() && keep(req.StateValue.IsNull(), req.PlanValue.IsUnknown(), req.ConfigValue.IsUnknown()) {
		resp.PlanValue = req.StateValue
	}
}

type keepStateMap struct{ keepState }

// KeepStateMap is a null-safe UseStateForUnknown for maps.
func KeepStateMap() planmodifier.Map { return keepStateMap{} }

func (keepStateMap) PlanModifyMap(_ context.Context, req planmodifier.MapRequest, resp *planmodifier.MapResponse) {
	if !req.State.Raw.IsNull() && keep(req.StateValue.IsNull(), req.PlanValue.IsUnknown(), req.ConfigValue.IsUnknown()) {
		resp.PlanValue = req.StateValue
	}
}

type keepStateObject struct{ keepState }

// KeepStateObject is a null-safe UseStateForUnknown for objects.
func KeepStateObject() planmodifier.Object { return keepStateObject{} }

func (keepStateObject) PlanModifyObject(_ context.Context, req planmodifier.ObjectRequest, resp *planmodifier.ObjectResponse) {
	if !req.State.Raw.IsNull() && keep(req.StateValue.IsNull(), req.PlanValue.IsUnknown(), req.ConfigValue.IsUnknown()) {
		resp.PlanValue = req.StateValue
	}
}
