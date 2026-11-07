package client

import (
	"context"
	"net/http"
	"testing"
	"time"
)

func TestEqFilter(t *testing.T) {
	cases := map[string]string{
		"portal":              `name eq "portal"`,
		"Sales and Marketing": `name eq "Sales and Marketing"`,
		"Partners or Vendors": `name eq "Partners or Vendors"`,
		`The "best" app`:      "",
		`back\slash`:          "",
		"":                    "",
	}
	for in, want := range cases {
		if got := EqFilter("name", in); got != want {
			t.Errorf("EqFilter(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestFindByEq(t *testing.T) {
	ctx := context.Background()
	all := []string{"a", "Read and Write", "b"}

	t.Run("filter works", func(t *testing.T) {
		var filters []string
		got, err := FindByEq(ctx, func(_ context.Context, f string) ([]string, error) {
			filters = append(filters, f)
			return []string{"a"}, nil
		}, "name", "a")
		if err != nil || len(got) != 1 || len(filters) != 1 || filters[0] != `name eq "a"` {
			t.Fatalf("got %v, %v, filters %q", got, err, filters)
		}
	})

	t.Run("bad request falls back to the full list", func(t *testing.T) {
		var filters []string
		got, err := FindByEq(ctx, func(_ context.Context, f string) ([]string, error) {
			filters = append(filters, f)
			if f != "" {
				return nil, &APIError{StatusCode: http.StatusBadRequest}
			}
			return all, nil
		}, "displayName", "Read and Write")
		if err != nil || len(got) != 3 || len(filters) != 2 || filters[1] != "" {
			t.Fatalf("got %v, %v, filters %q", got, err, filters)
		}
	})

	t.Run("other errors are returned", func(t *testing.T) {
		_, err := FindByEq(ctx, func(_ context.Context, f string) ([]string, error) {
			return nil, &APIError{StatusCode: http.StatusForbidden}
		}, "name", "a")
		if StatusCode(err) != http.StatusForbidden {
			t.Fatalf("got %v", err)
		}
	})

	t.Run("unquotable value lists everything", func(t *testing.T) {
		var filters []string
		_, _ = FindByEq(ctx, func(_ context.Context, f string) ([]string, error) {
			filters = append(filters, f)
			return all, nil
		}, "name", `say "hi"`)
		if len(filters) != 1 || filters[0] != "" {
			t.Fatalf("filters %q", filters)
		}
	})
}

func TestBackoffStaysPositiveAndCapped(t *testing.T) {
	for attempt := 0; attempt < 100; attempt++ {
		d := backoff(attempt, nil)
		if d <= 0 || d > 16*time.Second {
			t.Fatalf("backoff(%d) = %s", attempt, d)
		}
	}
}
