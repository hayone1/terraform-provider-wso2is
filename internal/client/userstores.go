package client

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// UserstoreID returns the API ID of a user store domain (unpadded base64url).
func UserstoreID(domain string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(domain))
}

// UserstoreType is a user store manager type.
type UserstoreType struct {
	TypeID    string `json:"typeId"`
	TypeName  string `json:"typeName"`
	ClassName string `json:"className"`
}

// ListUserstoreTypes lists the available user store manager types.
func (c *Client) ListUserstoreTypes(ctx context.Context) ([]UserstoreType, error) {
	var out []UserstoreType
	if err := c.Get(ctx, c.ServerPath("/userstores/meta/types"), nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// CreateUserstore creates a secondary user store and waits until the server reports it.
func (c *Client) CreateUserstore(ctx context.Context, body map[string]any) (string, error) {
	var out map[string]any
	resp, err := c.Post(ctx, c.ServerPath("/userstores"), body, &out)
	if err != nil {
		return "", err
	}
	id, _ := out["id"].(string)
	if id == "" {
		id = resp.LocationID()
	}
	if id == "" {
		name, _ := body["name"].(string)
		id = UserstoreID(name)
	}
	// User stores are deployed asynchronously.
	return id, c.waitUserstore(ctx, id, true)
}

// GetUserstore reads a user store.
func (c *Client) GetUserstore(ctx context.Context, id string) (map[string]any, error) {
	var out map[string]any
	if err := c.Get(ctx, c.ServerPath("/userstores/"+url.PathEscape(id)), nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// PutUserstore replaces a user store configuration.
func (c *Client) PutUserstore(ctx context.Context, id string, body map[string]any) error {
	_, err := c.Put(ctx, c.ServerPath("/userstores/"+url.PathEscape(id)), body, nil)
	return err
}

// DeleteUserstore deletes a user store and waits until it is gone.
func (c *Client) DeleteUserstore(ctx context.Context, id string) error {
	if err := c.Delete(ctx, c.ServerPath("/userstores/"+url.PathEscape(id))); err != nil {
		return err
	}
	return c.waitUserstore(ctx, id, false)
}

func (c *Client) waitUserstore(ctx context.Context, id string, present bool) error {
	deadline := time.Now().Add(2 * time.Minute)
	for {
		_, err := c.GetUserstore(ctx, id)
		switch {
		case present && err == nil, !present && IsNotFound(err):
			return nil
		case err != nil && !IsNotFound(err):
			return err
		}
		if time.Now().After(deadline) {
			state := "available"
			if !present {
				state = "removed"
			}
			return &APIError{Method: http.MethodGet, Path: "/userstores/" + id, StatusCode: http.StatusRequestTimeout, Message: fmt.Sprintf("user store was not %s within 2 minutes", state)}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
}
