package tfutil

import (
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// The helpers below build the attribute shapes used throughout the provider. WSO2 fills
// in defaults for most optional settings (and some are configurable per server), so the
// common shape is Optional+Computed with UseStateForUnknown: an omitted value takes the
// server's default and stays stable across plans.

// IDAttribute is the computed `id` attribute.
func IDAttribute(desc string) schema.StringAttribute {
	return schema.StringAttribute{
		MarkdownDescription: desc,
		Computed:            true,
		PlanModifiers:       []planmodifier.String{KeepStateString()},
	}
}

// ComputedString is a read-only string that keeps its prior value during plans.
func ComputedString(desc string) schema.StringAttribute {
	return schema.StringAttribute{
		MarkdownDescription: desc,
		Computed:            true,
		PlanModifiers:       []planmodifier.String{KeepStateString()},
	}
}

// OCString is an Optional+Computed string.
func OCString(desc string) schema.StringAttribute {
	return schema.StringAttribute{
		MarkdownDescription: desc,
		Optional:            true,
		Computed:            true,
		PlanModifiers:       []planmodifier.String{KeepStateString()},
	}
}

// OCBool is an Optional+Computed bool.
func OCBool(desc string) schema.BoolAttribute {
	return schema.BoolAttribute{
		MarkdownDescription: desc,
		Optional:            true,
		Computed:            true,
		PlanModifiers:       []planmodifier.Bool{KeepStateBool()},
	}
}

// OCInt64 is an Optional+Computed number.
func OCInt64(desc string) schema.Int64Attribute {
	return schema.Int64Attribute{
		MarkdownDescription: desc,
		Optional:            true,
		Computed:            true,
		PlanModifiers:       []planmodifier.Int64{KeepStateInt64()},
	}
}

// OCStringList is an Optional+Computed list of strings.
func OCStringList(desc string) schema.ListAttribute {
	return schema.ListAttribute{
		MarkdownDescription: desc,
		Optional:            true,
		Computed:            true,
		ElementType:         types.StringType,
		PlanModifiers:       []planmodifier.List{KeepStateList()},
	}
}

// OCStringSet is an Optional+Computed set of strings.
func OCStringSet(desc string) schema.SetAttribute {
	return schema.SetAttribute{
		MarkdownDescription: desc,
		Optional:            true,
		Computed:            true,
		ElementType:         types.StringType,
		PlanModifiers:       []planmodifier.Set{KeepStateSet()},
	}
}

// OCStringMap is an Optional+Computed map of strings.
func OCStringMap(desc string) schema.MapAttribute {
	return schema.MapAttribute{
		MarkdownDescription: desc,
		Optional:            true,
		Computed:            true,
		ElementType:         types.StringType,
		PlanModifiers:       []planmodifier.Map{KeepStateMap()},
	}
}

// OCObject is an Optional+Computed nested object.
func OCObject(desc string, attrs map[string]schema.Attribute) schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		MarkdownDescription: desc,
		Optional:            true,
		Computed:            true,
		Attributes:          attrs,
		PlanModifiers:       []planmodifier.Object{KeepStateObject()},
	}
}

// OCObjectList is an Optional+Computed list of nested objects.
func OCObjectList(desc string, attrs map[string]schema.Attribute) schema.ListNestedAttribute {
	return schema.ListNestedAttribute{
		MarkdownDescription: desc,
		Optional:            true,
		Computed:            true,
		NestedObject:        schema.NestedAttributeObject{Attributes: attrs},
		PlanModifiers:       []planmodifier.List{KeepStateList()},
	}
}

// OCObjectSet is an Optional+Computed set of nested objects.
func OCObjectSet(desc string, attrs map[string]schema.Attribute) schema.SetNestedAttribute {
	return schema.SetNestedAttribute{
		MarkdownDescription: desc,
		Optional:            true,
		Computed:            true,
		NestedObject:        schema.NestedAttributeObject{Attributes: attrs},
		PlanModifiers:       []planmodifier.Set{KeepStateSet()},
	}
}

// AttrTypes returns the attribute types of a nested attribute's object.
func AttrTypes(a schema.Attribute) map[string]attr.Type {
	switch t := a.GetType().(type) {
	case types.ObjectType:
		return t.AttrTypes
	case types.ListType:
		return t.ElemType.(types.ObjectType).AttrTypes
	case types.SetType:
		return t.ElemType.(types.ObjectType).AttrTypes
	}
	panic("tfutil.AttrTypes: not a nested attribute")
}

// NestedAttrTypes returns the attribute types of a nested attribute inside an object type.
func NestedAttrTypes(parent map[string]attr.Type, name string) map[string]attr.Type {
	switch t := parent[name].(type) {
	case types.ObjectType:
		return t.AttrTypes
	case types.ListType:
		return t.ElemType.(types.ObjectType).AttrTypes
	case types.SetType:
		return t.ElemType.(types.ObjectType).AttrTypes
	}
	panic("tfutil.NestedAttrTypes: " + name + " is not a nested attribute")
}
