package client

import (
	"reflect"
	"testing"
)

func TestMergeJSON(t *testing.T) {
	base := map[string]any{
		"clientId":    "abc",
		"grantTypes":  []any{"authorization_code", "refresh_token"},
		"accessToken": map[string]any{"type": "Default", "userAccessTokenExpiryInSeconds": 3600.0},
		"unmanaged":   "keep",
	}
	overlay := map[string]any{
		"grantTypes":  []any{"client_credentials"},
		"accessToken": map[string]any{"type": "JWT"},
		"logout":      map[string]any{"backChannelLogoutUrl": "https://x"},
	}
	got := MergeJSON(base, overlay)
	want := map[string]any{
		"clientId":    "abc",
		"grantTypes":  []any{"client_credentials"},
		"accessToken": map[string]any{"type": "JWT", "userAccessTokenExpiryInSeconds": 3600.0},
		"unmanaged":   "keep",
		"logout":      map[string]any{"backChannelLogoutUrl": "https://x"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("MergeJSON mismatch:\n got %#v\nwant %#v", got, want)
	}
}

func TestResponseLocationID(t *testing.T) {
	for loc, want := range map[string]string{
		"https://h/t/carbon.super/api/server/v1/applications/1234-abcd": "1234-abcd",
		"https://h/api/server/v1/userstores/U0VDT05EQVJZ/":              "U0VDT05EQVJZ",
		"/scim2/Users/a%20b": "a b",
		"":                   "",
	} {
		r := &Response{Header: map[string][]string{"Location": {loc}}}
		if loc == "" {
			r.Header = map[string][]string{}
		}
		if got := r.LocationID(); got != want {
			t.Errorf("LocationID(%q) = %q, want %q", loc, got, want)
		}
	}
}

func TestRedact(t *testing.T) {
	in := `{"clientSecret":"s3cr3t","password":"p","name":"app","properties":[{"key":"ClientSecret","value":"x"}]}`
	want := `{"clientSecret":"***","password":"***","name":"app","properties":[{"key":"ClientSecret","value":"***"}]}`
	if got := Redact(in); got != want {
		t.Fatalf("Redact:\n got %s\nwant %s", got, want)
	}
}

func TestPruneEmpty(t *testing.T) {
	overlay := map[string]any{
		"name":        "",
		"description": "",
		"accessToken": map[string]any{"bindingType": "", "type": "JWT"},
		"logout":      map[string]any{"backChannelLogoutUrl": ""},
	}
	base := map[string]any{
		"description": "old",
		"accessToken": map[string]any{"type": "Default"},
		"logout":      map[string]any{"backChannelLogoutUrl": "https://old"},
	}
	got := PruneEmpty(overlay, base)
	want := map[string]any{
		"description": "",
		"accessToken": map[string]any{"type": "JWT"},
		"logout":      map[string]any{"backChannelLogoutUrl": ""},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("PruneEmpty mismatch:\n got %#v\nwant %#v", got, want)
	}
}

func TestDiffJSON(t *testing.T) {
	prev := map[string]any{"a": "1", "b": []string{"x"}, "c": map[string]any{"d": true, "e": int64(5)}}
	next := map[string]any{"a": "1", "b": []string{"x", "y"}, "c": map[string]any{"d": true, "e": int64(6)}, "f": ""}
	got := DiffJSON(next, prev)
	want := map[string]any{"b": []string{"x", "y"}, "c": map[string]any{"e": int64(6)}, "f": ""}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("DiffJSON mismatch:\n got %#v\nwant %#v", got, want)
	}
}
