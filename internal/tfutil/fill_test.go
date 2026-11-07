package tfutil

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestSetIfNull(t *testing.T) {
	s := types.StringNull()
	SetStringIfNull(&s, "en-US")
	if s.ValueString() != "en-US" {
		t.Errorf("null string not filled: %s", s)
	}
	s = types.StringValue("fr-FR")
	SetStringIfNull(&s, "en-US")
	if s.ValueString() != "fr-FR" {
		t.Errorf("known string overwritten: %s", s)
	}
	b := types.BoolUnknown()
	SetBoolIfNull(&b, true)
	if !b.ValueBool() {
		t.Errorf("unknown bool not filled: %s", b)
	}
	b = types.BoolValue(false)
	SetBoolIfNull(&b, true)
	if b.ValueBool() {
		t.Errorf("known bool overwritten: %s", b)
	}
}

func TestIDParts(t *testing.T) {
	if got := IDParts(types.StringValue("ORG/carbon.super/en-US"), 3); len(got) != 3 || got[2] != "en-US" {
		t.Errorf("got %q", got)
	}
	if got := IDParts(types.StringValue("only-one"), 2); got != nil {
		t.Errorf("short ID: got %q, want nil", got)
	}
	if got := IDParts(types.StringNull(), 1); got != nil {
		t.Errorf("null ID: got %q, want nil", got)
	}
}
