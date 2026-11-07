package application

import "testing"

func TestCompatOIDC(t *testing.T) {
	body := func() map[string]any {
		return map[string]any{
			"cibaAuthenticationRequest": map[string]any{"authReqExpiryTime": 600},
			"accessToken":               map[string]any{"type": "JWT", "enableJwtScopeAsArray": false},
		}
	}
	// A 7.2 server returns neither key: both are dropped.
	got := compatOIDC(body(), map[string]any{"accessToken": map[string]any{"type": "Default"}})
	if _, ok := got["cibaAuthenticationRequest"]; ok {
		t.Error("CIBA settings sent to a server that does not know them")
	}
	if _, ok := got["accessToken"].(map[string]any)["enableJwtScopeAsArray"]; ok {
		t.Error("enableJwtScopeAsArray=false sent to a server that does not report it")
	}
	// A 7.3 server: CIBA is always returned; the scope-array flag is returned once set.
	cur := map[string]any{"cibaAuthenticationRequest": map[string]any{}, "accessToken": map[string]any{"enableJwtScopeAsArray": true}}
	got = compatOIDC(body(), cur)
	if _, ok := got["cibaAuthenticationRequest"]; !ok {
		t.Error("CIBA settings dropped for a server that supports them")
	}
	if v, ok := got["accessToken"].(map[string]any)["enableJwtScopeAsArray"]; !ok || v != false {
		t.Error("turning enableJwtScopeAsArray off must send false")
	}
}
