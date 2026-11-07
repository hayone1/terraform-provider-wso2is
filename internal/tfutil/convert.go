// Package tfutil holds helpers shared by the provider's resources and data sources:
// conversions between Terraform framework values and Go values, diagnostics for API
// errors, and common plan modifiers and validators.
package tfutil

import (
	"context"
	"sort"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// StringPtr returns nil for null/unknown values and a pointer otherwise.
func StringPtr(v types.String) *string {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	s := v.ValueString()
	return &s
}

// StringPtrNonEmpty is like StringPtr but also maps "" to nil.
func StringPtrNonEmpty(v types.String) *string {
	if v.IsNull() || v.IsUnknown() || v.ValueString() == "" {
		return nil
	}
	s := v.ValueString()
	return &s
}

// BoolPtr returns nil for null/unknown values and a pointer otherwise.
func BoolPtr(v types.Bool) *bool {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	b := v.ValueBool()
	return &b
}

// Int64Ptr returns nil for null/unknown values and a pointer otherwise.
func Int64Ptr(v types.Int64) *int64 {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	i := v.ValueInt64()
	return &i
}

// IntPtr returns nil for null/unknown values and a pointer otherwise.
func IntPtr(v types.Int64) *int {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	i := int(v.ValueInt64())
	return &i
}

// StringValue maps nil to null.
func StringValue(p *string) types.String {
	if p == nil {
		return types.StringNull()
	}
	return types.StringValue(*p)
}

// StringValueOrNull maps nil and "" to null.
func StringValueOrNull(s string) types.String {
	if s == "" {
		return types.StringNull()
	}
	return types.StringValue(s)
}

// BoolValue maps nil to null.
func BoolValue(p *bool) types.Bool {
	if p == nil {
		return types.BoolNull()
	}
	return types.BoolValue(*p)
}

// BoolValueOr maps nil to the given default.
func BoolValueOr(p *bool, def bool) types.Bool {
	if p == nil {
		return types.BoolValue(def)
	}
	return types.BoolValue(*p)
}

// Int64Value maps nil to null.
func Int64Value(p *int64) types.Int64 {
	if p == nil {
		return types.Int64Null()
	}
	return types.Int64Value(*p)
}

// IntValue maps nil to null.
func IntValue(p *int) types.Int64 {
	if p == nil {
		return types.Int64Null()
	}
	return types.Int64Value(int64(*p))
}

// Strings converts a list or set of strings to a Go slice. Null and unknown become nil.
func Strings(ctx context.Context, v interface {
	IsNull() bool
	IsUnknown() bool
	ElementsAs(context.Context, any, bool) diag.Diagnostics
}, diags *diag.Diagnostics) []string {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	var out []string
	diags.Append(v.ElementsAs(ctx, &out, false)...)
	return out
}

// ListOfStrings builds a list value; nil becomes an empty list unless nullIfEmpty is set.
func ListOfStrings(in []string, nullIfEmpty bool) types.List {
	if len(in) == 0 && nullIfEmpty {
		return types.ListNull(types.StringType)
	}
	elems := make([]attr.Value, 0, len(in))
	for _, s := range in {
		elems = append(elems, types.StringValue(s))
	}
	return types.ListValueMust(types.StringType, elems)
}

// SetOfStrings builds a set value (duplicates are dropped; some APIs repeat values); nil
// becomes an empty set unless nullIfEmpty is set.
func SetOfStrings(in []string, nullIfEmpty bool) types.Set {
	if len(in) == 0 && nullIfEmpty {
		return types.SetNull(types.StringType)
	}
	elems := make([]attr.Value, 0, len(in))
	seen := make(map[string]bool, len(in))
	for _, s := range in {
		if seen[s] {
			continue
		}
		seen[s] = true
		elems = append(elems, types.StringValue(s))
	}
	return types.SetValueMust(types.StringType, elems)
}

// MapOfStrings converts a map value to a Go map. Null and unknown become nil.
func MapOfStrings(ctx context.Context, v types.Map, diags *diag.Diagnostics) map[string]string {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	out := map[string]string{}
	diags.Append(v.ElementsAs(ctx, &out, false)...)
	return out
}

// MapValueOfStrings builds a map value; an empty map becomes null when nullIfEmpty is set.
func MapValueOfStrings(in map[string]string, nullIfEmpty bool) types.Map {
	if len(in) == 0 && nullIfEmpty {
		return types.MapNull(types.StringType)
	}
	elems := make(map[string]attr.Value, len(in))
	for k, s := range in {
		elems[k] = types.StringValue(s)
	}
	return types.MapValueMust(types.StringType, elems)
}

// SortedKeys returns the keys of m in order.
func SortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// Ptr returns a pointer to v.
func Ptr[T any](v T) *T { return &v }

// Deref returns *p or the zero value.
func Deref[T any](p *T) T {
	var zero T
	if p == nil {
		return zero
	}
	return *p
}

// Known reports whether v is neither null nor unknown.
func Known(v attr.Value) bool { return !v.IsNull() && !v.IsUnknown() }
