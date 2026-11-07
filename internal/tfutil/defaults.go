package tfutil

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/defaults"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Settings with a fixed server default use the helpers below: removing them from the
// configuration (or removing the whole block that holds them) puts the default back.
// Settings whose default depends on the server (token lifetimes, login flow, ...) stay
// Optional+Computed and keep their last value.

// Bool is an optional bool with a fixed default.
func Bool(desc string, def bool) schema.BoolAttribute {
	return schema.BoolAttribute{
		MarkdownDescription: desc + defaultSuffix(def),
		Optional:            true,
		Computed:            true,
		Default:             booldefault.StaticBool(def),
	}
}

// String is an optional string with a fixed default.
func String(desc, def string) schema.StringAttribute {
	return schema.StringAttribute{
		MarkdownDescription: desc + defaultSuffix(def),
		Optional:            true,
		Computed:            true,
		Default:             stringdefault.StaticString(def),
	}
}

// StringSet is an optional set of strings that defaults to empty.
func StringSet(desc string) schema.SetAttribute {
	return schema.SetAttribute{
		MarkdownDescription: desc,
		Optional:            true,
		Computed:            true,
		ElementType:         types.StringType,
		Default:             setdefault.StaticValue(types.SetValueMust(types.StringType, nil)),
	}
}

// StringList is an optional list of strings that defaults to empty.
func StringList(desc string) schema.ListAttribute {
	return schema.ListAttribute{
		MarkdownDescription: desc,
		Optional:            true,
		Computed:            true,
		ElementType:         types.StringType,
		Default:             listdefault.StaticValue(types.ListValueMust(types.StringType, nil)),
	}
}

func defaultSuffix(def any) string {
	switch v := def.(type) {
	case string:
		if v == "" {
			return ""
		}
		return " The default is `" + v + "`."
	case bool:
		if v {
			return " The default is `true`."
		}
		return " The default is `false`."
	}
	return ""
}

// Object is an optional nested object. When the configuration omits it, the plan keeps
// the prior state except for attributes with a default, which are reset to it — so
// removing a block behaves like removing each of its settings.
func Object(desc string, attrs map[string]schema.Attribute) schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		MarkdownDescription: desc,
		Optional:            true,
		Computed:            true,
		Attributes:          attrs,
		PlanModifiers:       []planmodifier.Object{objectDefaults{attrs: attrs}},
	}
}

type objectDefaults struct {
	attrs map[string]schema.Attribute
}

func (m objectDefaults) Description(context.Context) string {
	return "When not configured, keeps the prior value with defaulted settings reset."
}

func (m objectDefaults) MarkdownDescription(ctx context.Context) string { return m.Description(ctx) }

func (m objectDefaults) PlanModifyObject(ctx context.Context, req planmodifier.ObjectRequest, resp *planmodifier.ObjectResponse) {
	if !req.ConfigValue.IsNull() || req.StateValue.IsNull() || req.StateValue.IsUnknown() || !req.PlanValue.IsUnknown() {
		return
	}
	resp.PlanValue = withDefaults(ctx, m.attrs, req.StateValue)
}

// withDefaults returns obj with every attribute that has a static default set to it,
// recursing into nested objects.
func withDefaults(ctx context.Context, attrs map[string]schema.Attribute, obj types.Object) types.Object {
	if obj.IsNull() || obj.IsUnknown() {
		return obj
	}
	vals := obj.Attributes()
	out := make(map[string]attr.Value, len(vals))
	for name, v := range vals {
		out[name] = v
		a, ok := attrs[name]
		if !ok {
			continue
		}
		switch a := a.(type) {
		case schema.BoolAttribute:
			if a.Default != nil {
				var r defaults.BoolResponse
				a.Default.DefaultBool(ctx, defaults.BoolRequest{}, &r)
				out[name] = r.PlanValue
			}
		case schema.StringAttribute:
			if a.Default != nil {
				var r defaults.StringResponse
				a.Default.DefaultString(ctx, defaults.StringRequest{}, &r)
				out[name] = r.PlanValue
			}
		case schema.Int64Attribute:
			if a.Default != nil {
				var r defaults.Int64Response
				a.Default.DefaultInt64(ctx, defaults.Int64Request{}, &r)
				out[name] = r.PlanValue
			}
		case schema.SetAttribute:
			if a.Default != nil {
				var r defaults.SetResponse
				a.Default.DefaultSet(ctx, defaults.SetRequest{}, &r)
				out[name] = r.PlanValue
			}
		case schema.ListAttribute:
			if a.Default != nil {
				var r defaults.ListResponse
				a.Default.DefaultList(ctx, defaults.ListRequest{}, &r)
				out[name] = r.PlanValue
			}
		case schema.SetNestedAttribute:
			if a.Default != nil {
				var r defaults.SetResponse
				a.Default.DefaultSet(ctx, defaults.SetRequest{}, &r)
				out[name] = r.PlanValue
			}
		case schema.ListNestedAttribute:
			if a.Default != nil {
				var r defaults.ListResponse
				a.Default.DefaultList(ctx, defaults.ListRequest{}, &r)
				out[name] = r.PlanValue
			}
		case schema.SingleNestedAttribute:
			if child, ok := v.(types.Object); ok {
				out[name] = withDefaults(ctx, a.Attributes, child)
			}
		}
	}
	return finish(ctx, obj, attrs, vals, out)
}

func finish(ctx context.Context, obj types.Object, attrs map[string]schema.Attribute, vals, out map[string]attr.Value) types.Object {
	changed := false
	for name, v := range vals {
		if !out[name].Equal(v) {
			changed = true
		}
	}
	if changed {
		// Values the server derives from siblings cannot be predicted once a sibling
		// is reset: plan them as unknown.
		for name, a := range attrs {
			if isDerived(a) {
				out[name] = unknownOf(vals[name])
			}
		}
	}
	res, d := types.ObjectValue(obj.AttributeTypes(ctx), out)
	if d.HasError() {
		return obj
	}
	return res
}

func isDerived(a schema.Attribute) bool {
	var mods []any
	switch a := a.(type) {
	case schema.SetNestedAttribute:
		for _, m := range a.PlanModifiers {
			mods = append(mods, m)
		}
	case schema.SetAttribute:
		for _, m := range a.PlanModifiers {
			mods = append(mods, m)
		}
	case schema.StringAttribute:
		for _, m := range a.PlanModifiers {
			mods = append(mods, m)
		}
	case schema.BoolAttribute:
		for _, m := range a.PlanModifiers {
			mods = append(mods, m)
		}
	}
	for _, m := range mods {
		switch m.(type) {
		case setStateUnlessChanged, stringStateUnlessChanged, boolStateUnlessChanged:
			return true
		}
	}
	return false
}

func unknownOf(v attr.Value) attr.Value {
	switch t := v.(type) {
	case types.String:
		return types.StringUnknown()
	case types.Bool:
		return types.BoolUnknown()
	case types.Int64:
		return types.Int64Unknown()
	case types.Set:
		return types.SetUnknown(t.ElementType(context.Background()))
	case types.List:
		return types.ListUnknown(t.ElementType(context.Background()))
	case types.Object:
		return types.ObjectUnknown(t.AttributeTypes(context.Background()))
	}
	return v
}

// ObjectSet is an optional set of nested objects that defaults to empty.
func ObjectSet(desc string, attrs map[string]schema.Attribute) schema.SetNestedAttribute {
	return schema.SetNestedAttribute{
		MarkdownDescription: desc,
		Optional:            true,
		Computed:            true,
		NestedObject:        schema.NestedAttributeObject{Attributes: attrs},
		Default:             setdefault.StaticValue(types.SetValueMust(types.ObjectType{AttrTypes: objectAttrTypes(attrs)}, nil)),
	}
}

// ObjectList is an optional list of nested objects that defaults to empty.
func ObjectList(desc string, attrs map[string]schema.Attribute) schema.ListNestedAttribute {
	return schema.ListNestedAttribute{
		MarkdownDescription: desc,
		Optional:            true,
		Computed:            true,
		NestedObject:        schema.NestedAttributeObject{Attributes: attrs},
		Default:             listdefault.StaticValue(types.ListValueMust(types.ObjectType{AttrTypes: objectAttrTypes(attrs)}, nil)),
	}
}

func objectAttrTypes(attrs map[string]schema.Attribute) map[string]attr.Type {
	out := make(map[string]attr.Type, len(attrs))
	for k, a := range attrs {
		out[k] = a.GetType()
	}
	return out
}

// Int64 is an optional number with a fixed default.
func Int64(desc string, def int64) schema.Int64Attribute {
	return schema.Int64Attribute{
		MarkdownDescription: desc + fmt.Sprintf(" The default is `%d`.", def),
		Optional:            true,
		Computed:            true,
		Default:             int64default.StaticInt64(def),
	}
}
