package tfutil

import (
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Read must find the object from its ID and its arguments only. Some programs build the
// prior state that way and leave computed attributes, and attributes that have a default,
// empty: Terraform after an import, and Crossplane's Upjet, which runs this provider for
// Crossplane. The helpers below fill such attributes, usually from the parts of the ID.

// SetStringIfNull sets v to s when v is null or unknown.
func SetStringIfNull(v *types.String, s string) {
	if v.IsNull() || v.IsUnknown() {
		*v = types.StringValue(s)
	}
}

// SetBoolIfNull sets v to b when v is null or unknown.
func SetBoolIfNull(v *types.Bool, b bool) {
	if v.IsNull() || v.IsUnknown() {
		*v = types.BoolValue(b)
	}
}

// IDParts splits a composite ID at "/". It returns nil when the ID is null, unknown or
// has fewer than minParts parts.
func IDParts(id types.String, minParts int) []string {
	if !Known(id) {
		return nil
	}
	parts := strings.Split(id.ValueString(), "/")
	if len(parts) < minParts {
		return nil
	}
	return parts
}
