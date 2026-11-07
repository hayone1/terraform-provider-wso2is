package client

import (
	"encoding/json"
)

// ToMap converts a struct to a generic JSON object using its json tags.
func ToMap(v any) (map[string]any, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	out := map[string]any{}
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// FromMap decodes a generic JSON object into out.
func FromMap(m map[string]any, out any) error {
	b, err := json.Marshal(m)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, out)
}

// MergeJSON deep-merges overlay into base and returns base. Objects are merged key by
// key; any other value in overlay (including arrays and explicit nulls) replaces the
// value in base. It is used for read-modify-write updates of PUT endpoints, so that
// server-side fields the provider does not manage are sent back unchanged.
func MergeJSON(base, overlay map[string]any) map[string]any {
	if base == nil {
		base = map[string]any{}
	}
	for k, ov := range overlay {
		if om, ok := ov.(map[string]any); ok {
			if bm, ok := base[k].(map[string]any); ok {
				base[k] = MergeJSON(bm, om)
				continue
			}
		}
		base[k] = ov
	}
	return base
}

// PruneEmpty removes empty-string values from overlay (recursively) unless base holds a
// non-empty value for the same key, i.e. unless the empty string is meant to clear it.
// WSO2 omits unset optional strings from responses and rejects "" for several of them
// (for example the OIDC token binding type), so echoing them back must be avoided.
func PruneEmpty(overlay, base map[string]any) map[string]any {
	for k, v := range overlay {
		switch tv := v.(type) {
		case string:
			if tv == "" {
				if bv, ok := base[k].(string); !ok || bv == "" {
					delete(overlay, k)
				}
			}
		case map[string]any:
			bm, _ := base[k].(map[string]any)
			PruneEmpty(tv, bm)
			if len(tv) == 0 {
				delete(overlay, k)
			}
		}
	}
	return overlay
}

// DiffJSON returns the parts of next that differ from prev: objects are compared key by
// key (recursively), anything else (scalars, arrays) as a whole. Used to send only what
// changed in read-modify-write updates.
func DiffJSON(next, prev map[string]any) map[string]any {
	out := map[string]any{}
	for k, nv := range next {
		pv, ok := prev[k]
		if nm, isMap := nv.(map[string]any); isMap {
			pm, _ := pv.(map[string]any)
			if d := DiffJSON(nm, pm); len(d) > 0 {
				out[k] = d
			}
			continue
		}
		if !ok || !jsonEqual(nv, pv) {
			out[k] = nv
		}
	}
	return out
}

func jsonEqual(a, b any) bool {
	ab, err1 := json.Marshal(a)
	bb, err2 := json.Marshal(b)
	return err1 == nil && err2 == nil && string(ab) == string(bb)
}

// JSONEqualString reports whether two JSON documents are equal, ignoring formatting and
// key order. Invalid JSON is compared as text.
func JSONEqualString(a, b string) bool {
	var av, bv any
	if json.Unmarshal([]byte(a), &av) != nil || json.Unmarshal([]byte(b), &bv) != nil {
		return a == b
	}
	return jsonEqual(av, bv)
}

// CloneJSON returns a deep copy of a JSON object.
func CloneJSON(m map[string]any) map[string]any {
	b, err := json.Marshal(m)
	if err != nil {
		return map[string]any{}
	}
	out := map[string]any{}
	_ = json.Unmarshal(b, &out)
	return out
}
