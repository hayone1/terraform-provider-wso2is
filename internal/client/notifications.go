package client

import (
	"context"
	"encoding/base64"
	"net/url"
)

// Notification senders are singletons per tenant: EmailPublisher and SMSPublisher.
const (
	EmailPublisher = "EmailPublisher"
	SMSPublisher   = "SMSPublisher"
)

func senderPath(c *Client, channel, name string) string {
	p := c.ServerV2Path("/notification-senders/" + channel)
	if name != "" {
		p += "/" + url.PathEscape(name)
	}
	return p
}

// CreateNotificationSender creates the email or SMS sender (channel `email` or `sms`).
func (c *Client) CreateNotificationSender(ctx context.Context, channel string, body map[string]any) error {
	_, err := c.Post(ctx, senderPath(c, channel, ""), body, nil)
	return err
}

// GetNotificationSender reads a sender.
func (c *Client) GetNotificationSender(ctx context.Context, channel, name string) (map[string]any, error) {
	var out map[string]any
	if err := c.Get(ctx, senderPath(c, channel, name), nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// UpdateNotificationSender replaces a sender (the name cannot change).
func (c *Client) UpdateNotificationSender(ctx context.Context, channel, name string, body map[string]any) error {
	_, err := c.Put(ctx, senderPath(c, channel, name), body, nil)
	return err
}

// DeleteNotificationSender deletes a sender.
func (c *Client) DeleteNotificationSender(ctx context.Context, channel, name string) error {
	return c.Delete(ctx, senderPath(c, channel, name))
}

// TemplateTypeID returns the API ID of a notification template type name.
func TemplateTypeID(name string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(name))
}

// NotificationTemplatePath returns the path of a template collection or, with locale,
// of one template. appID selects an application-specific template.
func (c *Client) NotificationTemplatePath(channel, typeName, appID, locale string) string {
	p := c.ServerPath("/notification/" + channel + "/template-types/" + url.PathEscape(TemplateTypeID(typeName)))
	if appID != "" {
		p += "/app-templates/" + url.PathEscape(appID)
	} else {
		p += "/org-templates"
	}
	if locale != "" {
		p += "/" + url.PathEscape(locale)
	}
	return p
}

// CreateTemplateType creates a custom notification template type.
func (c *Client) CreateTemplateType(ctx context.Context, channel, name string) error {
	_, err := c.Post(ctx, c.ServerPath("/notification/"+channel+"/template-types"), map[string]string{"displayName": name}, nil)
	return err
}

// GetTemplateType reads a template type.
func (c *Client) GetTemplateType(ctx context.Context, channel, name string) (map[string]any, error) {
	var out map[string]any
	if err := c.Get(ctx, c.ServerPath("/notification/"+channel+"/template-types/"+url.PathEscape(TemplateTypeID(name))), nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// DeleteTemplateType deletes a custom template type and its templates.
func (c *Client) DeleteTemplateType(ctx context.Context, channel, name string) error {
	return c.Delete(ctx, c.ServerPath("/notification/"+channel+"/template-types/"+url.PathEscape(TemplateTypeID(name))))
}
