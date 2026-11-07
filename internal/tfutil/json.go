package tfutil

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

// JSON is a JSON object under construction. The setters only write known values, so a
// value the user left to the server's default is not sent.
type JSON map[string]any

// Str sets k when v is known.
func (o JSON) Str(k string, v types.String) JSON {
	if Known(v) {
		o[k] = v.ValueString()
	}
	return o
}

// Bool sets k when v is known.
func (o JSON) Bool(k string, v types.Bool) JSON {
	if Known(v) {
		o[k] = v.ValueBool()
	}
	return o
}

// Int sets k when v is known.
func (o JSON) Int(k string, v types.Int64) JSON {
	if Known(v) {
		o[k] = v.ValueInt64()
	}
	return o
}

// Strs sets k to the elements of a known list or set.
func (o JSON) Strs(ctx context.Context, k string, v interface {
	IsNull() bool
	IsUnknown() bool
	ElementsAs(context.Context, any, bool) diag.Diagnostics
}, diags *diag.Diagnostics) JSON {
	if v.IsNull() || v.IsUnknown() {
		return o
	}
	s := Strings(ctx, v, diags)
	if s == nil {
		s = []string{}
	}
	o[k] = s
	return o
}

// Obj sets k to sub when sub has at least one key.
func (o JSON) Obj(k string, sub JSON) JSON {
	if len(sub) > 0 {
		o[k] = map[string]any(sub)
	}
	return o
}

// Map returns the plain map.
func (o JSON) Map() map[string]any { return map[string]any(o) }

// Getters for decoded JSON (map[string]any from encoding/json).

// GetMap returns the nested object at k, or nil.
func GetMap(m map[string]any, k string) map[string]any {
	if m == nil {
		return nil
	}
	v, _ := m[k].(map[string]any)
	return v
}

// GetString returns the string at k as a framework value; missing becomes null.
func GetString(m map[string]any, k string) types.String {
	if m == nil {
		return types.StringNull()
	}
	switch v := m[k].(type) {
	case string:
		return types.StringValue(v)
	case nil:
		return types.StringNull()
	case float64:
		return types.StringValue(strconv.FormatFloat(v, 'f', -1, 64))
	case bool:
		return types.StringValue(strconv.FormatBool(v))
	default:
		return types.StringValue(fmt.Sprint(v))
	}
}

// GetStringOrEmpty is GetString but missing becomes "".
func GetStringOrEmpty(m map[string]any, k string) types.String {
	v := GetString(m, k)
	if v.IsNull() {
		return types.StringValue("")
	}
	return v
}

// GetBool returns the bool at k; missing becomes def.
func GetBool(m map[string]any, k string, def bool) types.Bool {
	if m == nil {
		return types.BoolValue(def)
	}
	switch v := m[k].(type) {
	case bool:
		return types.BoolValue(v)
	case string:
		b, err := strconv.ParseBool(v)
		if err == nil {
			return types.BoolValue(b)
		}
	}
	return types.BoolValue(def)
}

// GetInt64 returns the number at k; missing becomes null.
func GetInt64(m map[string]any, k string) types.Int64 {
	if m == nil {
		return types.Int64Null()
	}
	switch v := m[k].(type) {
	case float64:
		return types.Int64Value(int64(math.Round(v)))
	case json.Number:
		i, err := v.Int64()
		if err == nil {
			return types.Int64Value(i)
		}
	case string:
		i, err := strconv.ParseInt(v, 10, 64)
		if err == nil {
			return types.Int64Value(i)
		}
	}
	return types.Int64Null()
}

// GetStrings returns the string array at k (empty when missing).
func GetStrings(m map[string]any, k string) []string {
	if m == nil {
		return []string{}
	}
	arr, _ := m[k].([]any)
	out := make([]string, 0, len(arr))
	for _, e := range arr {
		if s, ok := e.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

// GetObjects returns the object array at k.
func GetObjects(m map[string]any, k string) []map[string]any {
	if m == nil {
		return nil
	}
	arr, _ := m[k].([]any)
	out := make([]map[string]any, 0, len(arr))
	for _, e := range arr {
		if o, ok := e.(map[string]any); ok {
			out = append(out, o)
		}
	}
	return out
}

// ObjectAs decodes a known object into a model struct. It returns false for null or
// unknown objects.
func ObjectAs[T any](ctx context.Context, obj types.Object, diags *diag.Diagnostics) (*T, bool) {
	if obj.IsNull() || obj.IsUnknown() {
		return nil, false
	}
	var out T
	diags.Append(obj.As(ctx, &out, basetypes.ObjectAsOptions{UnhandledUnknownAsEmpty: false})...)
	return &out, !diags.HasError()
}

// ElementsAs decodes a known list or set of objects into model structs.
func ElementsAs[T any](ctx context.Context, v interface {
	IsNull() bool
	IsUnknown() bool
	ElementsAs(context.Context, any, bool) diag.Diagnostics
}, diags *diag.Diagnostics) ([]T, bool) {
	if v.IsNull() || v.IsUnknown() {
		return nil, false
	}
	var out []T
	diags.Append(v.ElementsAs(ctx, &out, false)...)
	return out, !diags.HasError()
}

// ObjectFrom builds an object value from a model struct.
func ObjectFrom(ctx context.Context, types_ map[string]attr.Type, model any, diags *diag.Diagnostics) types.Object {
	v, d := types.ObjectValueFrom(ctx, types_, model)
	diags.Append(d...)
	return v
}

// ListFrom builds a list of objects from model structs.
func ListFrom(ctx context.Context, types_ map[string]attr.Type, models any, diags *diag.Diagnostics) types.List {
	v, d := types.ListValueFrom(ctx, types.ObjectType{AttrTypes: types_}, models)
	diags.Append(d...)
	return v
}

// SetFrom builds a set of objects from model structs.
func SetFrom(ctx context.Context, types_ map[string]attr.Type, models any, diags *diag.Diagnostics) types.Set {
	v, d := types.SetValueFrom(ctx, types.ObjectType{AttrTypes: types_}, models)
	diags.Append(d...)
	return v
}
