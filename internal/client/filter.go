package client

import (
	"context"
	"net/http"
	"strings"
)

// EqFilter returns a filter expression that matches attr equal to value, for the
// `filter` query parameter of the server and SCIM list APIs. The value is quoted, so
// names with spaces or with the words "and" and "or" (for example "Sales and Marketing")
// parse as one value. EqFilter returns "" when the value contains a double quote or a
// backslash: the server's filter parsers cannot express those, and an escaped quote
// silently matches nothing.
func EqFilter(attr, value string) string {
	if value == "" || strings.ContainsAny(value, `"\`) {
		return ""
	}
	return attr + ` eq "` + value + `"`
}

// FindByEq lists the objects whose attr equals value with list. When the value cannot
// be put in a filter, or the server cannot evaluate the filter (HTTP 400, for example
// the roles API with a name that contains "and"), it lists every object instead.
// Callers must still compare the results exactly: some filters ignore case, and the
// fallback returns everything.
func FindByEq[T any](ctx context.Context, list func(ctx context.Context, filter string) ([]T, error), attr, value string) ([]T, error) {
	if f := EqFilter(attr, value); f != "" {
		items, err := list(ctx, f)
		if err == nil || StatusCode(err) != http.StatusBadRequest {
			return items, err
		}
	}
	return list(ctx, "")
}
