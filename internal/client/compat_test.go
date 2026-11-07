package client

import "testing"

func TestCloneJSONIsDeep(t *testing.T) {
	orig := map[string]any{"accessToken": map[string]any{"type": "JWT"}}
	c := CloneJSON(orig)
	MergeJSON(orig, map[string]any{"accessToken": map[string]any{"enableJWTScopeAsArray": false}})
	if _, ok := c["accessToken"].(map[string]any)["enableJWTScopeAsArray"]; ok {
		t.Fatal("clone shares nested maps with the original")
	}
}

func TestJSONEqualString(t *testing.T) {
	if !JSONEqualString(`{"a":1,"b":[1,2]}`, "{ \"b\": [1, 2], \"a\": 1 }") {
		t.Fatal("formatting and key order should not matter")
	}
	if JSONEqualString(`{"a":1}`, `{"a":2}`) {
		t.Fatal("different values reported equal")
	}
}
