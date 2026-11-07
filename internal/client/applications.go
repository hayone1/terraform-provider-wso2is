package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// Application is the application model used for create, read and patch. Pointer and
// omitempty fields let callers send only what they manage.
type Application struct {
	ID                               string                    `json:"id,omitempty"`
	Name                             string                    `json:"name,omitempty"`
	Description                      *string                   `json:"description,omitempty"`
	ApplicationVersion               string                    `json:"applicationVersion,omitempty"`
	ImageURL                         *string                   `json:"imageUrl,omitempty"`
	AccessURL                        *string                   `json:"accessUrl,omitempty"`
	LogoutReturnURL                  *string                   `json:"logoutReturnUrl,omitempty"`
	ClientID                         string                    `json:"clientId,omitempty"`
	Issuer                           string                    `json:"issuer,omitempty"`
	Realm                            string                    `json:"realm,omitempty"`
	TemplateID                       *string                   `json:"templateId,omitempty"`
	TemplateVersion                  *string                   `json:"templateVersion,omitempty"`
	IsManagementApp                  *bool                     `json:"isManagementApp,omitempty"`
	IsB2BSelfServiceApp              *bool                     `json:"isB2BSelfServiceApp,omitempty"`
	EnhancedOrgAuthenticationEnabled *bool                     `json:"enhancedOrgAuthenticationEnabled,omitempty"`
	ApplicationEnabled               *bool                     `json:"applicationEnabled,omitempty"`
	AssociatedRoles                  *AssociatedRolesConfig    `json:"associatedRoles,omitempty"`
	ClaimConfiguration               *ClaimConfiguration       `json:"claimConfiguration,omitempty"`
	InboundProtocolConfiguration     *InboundProtocols         `json:"inboundProtocolConfiguration,omitempty"`
	AuthenticationSequence           *AuthenticationSequence   `json:"authenticationSequence,omitempty"`
	AdvancedConfigurations           *AdvancedAppConfiguration `json:"advancedConfigurations,omitempty"`
	ProvisioningConfigurations       *AppProvisioning          `json:"provisioningConfigurations,omitempty"`
	InboundProtocols                 []InboundProtocolListItem `json:"inboundProtocols,omitempty"`
	Access                           string                    `json:"access,omitempty"`
}

type InboundProtocolListItem struct {
	Type string `json:"type"`
	Name string `json:"name,omitempty"`
	Self string `json:"self,omitempty"`
}

type AssociatedRolesConfig struct {
	AllowedAudience string    `json:"allowedAudience"`
	Roles           []RoleRef `json:"roles"`
}

type RoleRef struct {
	ID   string `json:"id"`
	Name string `json:"name,omitempty"`
}

type ClaimConfiguration struct {
	Dialect         string              `json:"dialect,omitempty"`
	ClaimMappings   []AppClaimMapping   `json:"claimMappings"`
	RequestedClaims []AppRequestedClaim `json:"requestedClaims"`
	Subject         *AppSubjectConfig   `json:"subject,omitempty"`
	Role            *AppRoleClaimConfig `json:"role,omitempty"`
}

type ClaimRef struct {
	ID          string `json:"id,omitempty"`
	URI         string `json:"uri"`
	DisplayName string `json:"displayName,omitempty"`
}

type AppClaimMapping struct {
	ApplicationClaim string   `json:"applicationClaim"`
	LocalClaim       ClaimRef `json:"localClaim"`
}

type AppRequestedClaim struct {
	Claim     ClaimRef `json:"claim"`
	Mandatory bool     `json:"mandatory"`
}

type AppSubjectConfig struct {
	Claim                       *ClaimRef `json:"claim,omitempty"`
	IncludeUserDomain           *bool     `json:"includeUserDomain,omitempty"`
	IncludeTenantDomain         *bool     `json:"includeTenantDomain,omitempty"`
	UseMappedLocalSubject       *bool     `json:"useMappedLocalSubject,omitempty"`
	MappedLocalSubjectMandatory *bool     `json:"mappedLocalSubjectMandatory,omitempty"`
}

type AppRoleClaimConfig struct {
	Mappings          []AppRoleMapping `json:"mappings,omitempty"`
	IncludeUserDomain *bool            `json:"includeUserDomain,omitempty"`
	Claim             *ClaimRef        `json:"claim,omitempty"`
}

type AppRoleMapping struct {
	LocalRole       string `json:"localRole"`
	ApplicationRole string `json:"applicationRole"`
}

type AuthenticationSequence struct {
	Type                      string               `json:"type,omitempty"`
	Steps                     []AuthenticationStep `json:"steps"`
	RequestPathAuthenticators []string             `json:"requestPathAuthenticators"`
	Script                    *string              `json:"script,omitempty"`
	SubjectStepID             *int64               `json:"subjectStepId,omitempty"`
	AttributeStepID           *int64               `json:"attributeStepId,omitempty"`
}

type AuthenticationStep struct {
	ID      int64              `json:"id"`
	Options []AuthenticatorRef `json:"options"`
}

type AuthenticatorRef struct {
	IDP           string `json:"idp"`
	Authenticator string `json:"authenticator"`
}

type AdvancedAppConfiguration struct {
	SaaS                         *bool                    `json:"saas,omitempty"`
	DiscoverableByEndUsers       *bool                    `json:"discoverableByEndUsers,omitempty"`
	DiscoverableGroups           []DiscoverableGroup      `json:"discoverableGroups,omitempty"`
	Certificate                  *AppCertificate          `json:"certificate,omitempty"`
	SkipLoginConsent             *bool                    `json:"skipLoginConsent,omitempty"`
	SkipLogoutConsent            *bool                    `json:"skipLogoutConsent,omitempty"`
	UseExternalConsentPage       *bool                    `json:"useExternalConsentPage,omitempty"`
	ReturnAuthenticatedIdpList   *bool                    `json:"returnAuthenticatedIdpList,omitempty"`
	EnableAuthorization          *bool                    `json:"enableAuthorization,omitempty"`
	Fragment                     *bool                    `json:"fragment,omitempty"`
	EnableAPIBasedAuthentication *bool                    `json:"enableAPIBasedAuthentication,omitempty"`
	AttestationMetaData          *AttestationMetaData     `json:"attestationMetaData,omitempty"`
	TrustedAppConfiguration      *TrustedAppConfiguration `json:"trustedAppConfiguration,omitempty"`
	AdditionalSpProperties       []AdditionalSpProperty   `json:"additionalSpProperties,omitempty"`
}

type DiscoverableGroup struct {
	UserStore string           `json:"userStore"`
	Groups    []GroupBasicInfo `json:"groups"`
}

type GroupBasicInfo struct {
	ID   string `json:"id"`
	Name string `json:"name,omitempty"`
}

type AppCertificate struct {
	Type  string `json:"type,omitempty"`
	Value string `json:"value,omitempty"`
}

type AttestationMetaData struct {
	EnableClientAttestation              *bool           `json:"enableClientAttestation,omitempty"`
	AndroidPackageName                   *string         `json:"androidPackageName,omitempty"`
	AndroidAttestationServiceCredentials json.RawMessage `json:"androidAttestationServiceCredentials,omitempty"`
	AppleAppID                           *string         `json:"appleAppId,omitempty"`
}

type TrustedAppConfiguration struct {
	IsFIDOTrustedApp   *bool    `json:"isFIDOTrustedApp,omitempty"`
	IsConsentGranted   *bool    `json:"isConsentGranted,omitempty"`
	AndroidPackageName *string  `json:"androidPackageName,omitempty"`
	AndroidThumbprints []string `json:"androidThumbprints,omitempty"`
	AppleAppID         *string  `json:"appleAppId,omitempty"`
}

type AdditionalSpProperty struct {
	Name        string `json:"name"`
	Value       string `json:"value"`
	DisplayName string `json:"displayName,omitempty"`
}

type AppProvisioning struct {
	InboundProvisioning      *InboundSCIMProvisioning  `json:"inboundProvisioning,omitempty"`
	OutboundProvisioningIdps []OutboundProvisioningIdp `json:"outboundProvisioningIdps"`
}

type InboundSCIMProvisioning struct {
	ProxyMode                   *bool   `json:"proxyMode,omitempty"`
	ProvisioningUserstoreDomain *string `json:"provisioningUserstoreDomain,omitempty"`
}

type OutboundProvisioningIdp struct {
	IDP       string `json:"idp"`
	Connector string `json:"connector"`
	Blocking  bool   `json:"blocking"`
	Rules     bool   `json:"rules"`
	JIT       bool   `json:"jit"`
}

type InboundProtocols struct {
	SAML       *SAMLInbound      `json:"saml,omitempty"`
	OIDC       map[string]any    `json:"oidc,omitempty"`
	PassiveSTS *PassiveSTSConfig `json:"passiveSts,omitempty"`
	WSTrust    *WSTrustConfig    `json:"wsTrust,omitempty"`
}

// SAMLInbound is the body of a SAML inbound protocol create/update.
type SAMLInbound struct {
	MetadataFile        string         `json:"metadataFile,omitempty"`
	MetadataURL         string         `json:"metadataURL,omitempty"`
	ManualConfiguration map[string]any `json:"manualConfiguration,omitempty"`
}

type PassiveSTSConfig struct {
	Realm         string `json:"realm"`
	ReplyTo       string `json:"replyTo"`
	ReplyToLogout string `json:"replyToLogout,omitempty"`
}

type WSTrustConfig struct {
	Audience         string `json:"audience"`
	CertificateAlias string `json:"certificateAlias"`
}

// ApplicationListItem is one entry of GET /applications.
type ApplicationListItem struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Description     string `json:"description"`
	ClientID        string `json:"clientId"`
	Issuer          string `json:"issuer"`
	Realm           string `json:"realm"`
	AccessURL       string `json:"accessUrl"`
	TemplateID      string `json:"templateId"`
	Access          string `json:"access"`
	IsManagementApp bool   `json:"isManagementApp"`
}

type applicationList struct {
	TotalResults int                   `json:"totalResults"`
	StartIndex   int                   `json:"startIndex"`
	Count        int                   `json:"count"`
	Applications []ApplicationListItem `json:"applications"`
}

func (c *Client) appPath(id string) string {
	return c.ServerPath("/applications/" + url.PathEscape(id))
}

// CreateApplication creates an application and returns its ID.
func (c *Client) CreateApplication(ctx context.Context, app *Application) (string, error) {
	resp, err := c.Do(ctx, Request{Method: http.MethodPost, Path: c.ServerPath("/applications"), Body: app})
	if err != nil {
		return "", err
	}
	id := resp.LocationID()
	if id == "" {
		return "", fmt.Errorf("application created but the server did not return its location")
	}
	return id, nil
}

// GetApplication reads an application.
func (c *Client) GetApplication(ctx context.Context, id string) (*Application, error) {
	var app Application
	if err := c.Get(ctx, c.appPath(id), nil, &app); err != nil {
		return nil, err
	}
	return &app, nil
}

// ApplicationMissing reports whether the application appID does not exist. Several
// endpoints below an application do not answer 404 when the application was deleted:
// authorized-apis answers 400 APP-60001 ("does not belong to the tenant domain"),
// share answers 200 with no organizations, and shared-apps answers 400 ORG-60035.
// Callers use this to tell a deleted application from a real error.
//
// After a delete, the server keeps answering GET /applications/{id} with the old
// application for about 15 minutes (a cache), while the endpoints below the application
// already know that it is gone. When GET still finds the application, the authorized-apis
// endpoint decides.
func (c *Client) ApplicationMissing(ctx context.Context, appID string) bool {
	_, err := c.GetApplication(ctx, appID)
	if IsNotFound(err) {
		return true
	}
	if err != nil {
		return false
	}
	_, err = c.ListAuthorizedAPIs(ctx, appID)
	return applicationNotInTenant(err)
}

// ApplicationGone reports whether err, which an endpoint below the application appID
// returned, means that the application does not exist. The answers that only a missing
// application gives are checked first, because GET /applications/{id} can still find a
// deleted application (see ApplicationMissing).
func (c *Client) ApplicationGone(ctx context.Context, appID string, err error) bool {
	if err == nil {
		return false
	}
	if IsNotFound(err) || applicationNotInTenant(err) || HasCode(err, "ORG-60035") {
		return true
	}
	return c.ApplicationMissing(ctx, appID)
}

// applicationNotInTenant reports whether err is the server's answer for an application that
// is not in the tenant: 400 APP-60001 "Application does not belong to the tenant domain".
// APP-60001 alone also covers other failures, so the message decides.
func applicationNotInTenant(err error) bool {
	return HasCode(err, "APP-60001") && strings.Contains(err.Error(), "does not belong to the tenant domain")
}

// PatchApplication updates the fields set in patch.
func (c *Client) PatchApplication(ctx context.Context, id string, patch *Application) error {
	_, err := c.Patch(ctx, c.appPath(id), patch, nil)
	return err
}

// DeleteApplication deletes an application.
func (c *Client) DeleteApplication(ctx context.Context, id string) error {
	return c.Delete(ctx, c.appPath(id))
}

// ListApplications returns every application matching the optional SCIM-style filter
// (for example `name eq My App` or `clientId eq abc`), following pagination.
func (c *Client) ListApplications(ctx context.Context, filter string) ([]ApplicationListItem, error) {
	var out []ApplicationListItem
	const limit = 100
	for offset := 0; ; offset += limit {
		q := url.Values{"limit": {fmt.Sprint(limit)}, "offset": {fmt.Sprint(offset)}}
		if filter != "" {
			q.Set("filter", filter)
		}
		var page applicationList
		if err := c.Get(ctx, c.ServerPath("/applications"), q, &page); err != nil {
			return nil, err
		}
		out = append(out, page.Applications...)
		if len(page.Applications) < limit || (page.TotalResults > 0 && len(out) >= page.TotalResults) {
			return out, nil
		}
	}
}

// GetInboundOIDC returns the OIDC configuration as a generic map, so that fields this
// provider does not model survive a read-modify-write update.
func (c *Client) GetInboundOIDC(ctx context.Context, appID string) (map[string]any, error) {
	var out map[string]any
	if err := c.Get(ctx, c.appPath(appID)+"/inbound-protocols/oidc", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// PutInboundOIDC replaces the OIDC configuration.
func (c *Client) PutInboundOIDC(ctx context.Context, appID string, cfg map[string]any) error {
	_, err := c.Put(ctx, c.appPath(appID)+"/inbound-protocols/oidc", cfg, nil)
	return err
}

// DeleteInboundProtocol removes an inbound protocol (oidc, saml, passive-sts, ws-trust).
func (c *Client) DeleteInboundProtocol(ctx context.Context, appID, protocol string) error {
	return c.Delete(ctx, c.appPath(appID)+"/inbound-protocols/"+protocol)
}

// RegenerateOIDCSecret issues a new client secret.
func (c *Client) RegenerateOIDCSecret(ctx context.Context, appID string) (map[string]any, error) {
	var out map[string]any
	_, err := c.Post(ctx, c.appPath(appID)+"/inbound-protocols/oidc/regenerate-secret", nil, &out)
	return out, err
}

// GetInboundSAML returns the SAML service provider configuration.
func (c *Client) GetInboundSAML(ctx context.Context, appID string) (map[string]any, error) {
	var out map[string]any
	if err := c.Get(ctx, c.appPath(appID)+"/inbound-protocols/saml", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// PutInboundSAML replaces the SAML configuration.
func (c *Client) PutInboundSAML(ctx context.Context, appID string, cfg *SAMLInbound) error {
	_, err := c.Put(ctx, c.appPath(appID)+"/inbound-protocols/saml", cfg, nil)
	return err
}

// GetInboundPassiveSTS returns the WS-Federation (passive STS) configuration.
func (c *Client) GetInboundPassiveSTS(ctx context.Context, appID string) (*PassiveSTSConfig, error) {
	var out PassiveSTSConfig
	if err := c.Get(ctx, c.appPath(appID)+"/inbound-protocols/passive-sts", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// PutInboundPassiveSTS replaces the passive STS configuration.
func (c *Client) PutInboundPassiveSTS(ctx context.Context, appID string, cfg *PassiveSTSConfig) error {
	_, err := c.Put(ctx, c.appPath(appID)+"/inbound-protocols/passive-sts", cfg, nil)
	return err
}

// GetInboundWSTrust returns the WS-Trust configuration.
func (c *Client) GetInboundWSTrust(ctx context.Context, appID string) (*WSTrustConfig, error) {
	var out WSTrustConfig
	if err := c.Get(ctx, c.appPath(appID)+"/inbound-protocols/ws-trust", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// PutInboundWSTrust replaces the WS-Trust configuration.
func (c *Client) PutInboundWSTrust(ctx context.Context, appID string, cfg *WSTrustConfig) error {
	_, err := c.Put(ctx, c.appPath(appID)+"/inbound-protocols/ws-trust", cfg, nil)
	return err
}

// PutAuthenticationScript replaces only the adaptive authentication script.
func (c *Client) PutAuthenticationScript(ctx context.Context, appID, script string) error {
	_, err := c.Put(ctx, c.appPath(appID)+"/authenticationSequence/script", map[string]string{"script": script}, nil)
	return err
}

// AuthorizedAPI is an API resource authorized for an application.
type AuthorizedAPI struct {
	ID                                  string            `json:"id"`
	Identifier                          string            `json:"identifier,omitempty"`
	DisplayName                         string            `json:"displayName,omitempty"`
	PolicyID                            string            `json:"policyId,omitempty"`
	Type                                string            `json:"type,omitempty"`
	AuthorizedScopes                    []AuthorizedScope `json:"authorizedScopes,omitempty"`
	AuthorizedAuthorizationDetailsTypes []struct {
		ID   string `json:"id"`
		Type string `json:"type"`
		Name string `json:"name"`
	} `json:"authorizedAuthorizationDetailsTypes,omitempty"`
}

type AuthorizedScope struct {
	ID          string `json:"id,omitempty"`
	Name        string `json:"name"`
	DisplayName string `json:"displayName,omitempty"`
}

// AuthorizeAPI authorizes an API resource (with scopes) for an application.
func (c *Client) AuthorizeAPI(ctx context.Context, appID, apiID, policy string, scopes, authzDetailsTypes []string) error {
	body := map[string]any{"id": apiID, "policyIdentifier": policy, "scopes": nonNil(scopes)}
	if len(authzDetailsTypes) > 0 {
		body["authorizationDetailsTypes"] = authzDetailsTypes
	}
	_, err := c.Post(ctx, c.appPath(appID)+"/authorized-apis", body, nil)
	return err
}

// ListAuthorizedAPIs returns the API resources authorized for an application.
func (c *Client) ListAuthorizedAPIs(ctx context.Context, appID string) ([]AuthorizedAPI, error) {
	var out []AuthorizedAPI
	if err := c.Get(ctx, c.appPath(appID)+"/authorized-apis", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// PatchAuthorizedAPI adds and removes authorized scopes.
func (c *Client) PatchAuthorizedAPI(ctx context.Context, appID, apiID string, added, removed []string) error {
	body := map[string]any{"addedScopes": nonNil(added), "removedScopes": nonNil(removed)}
	_, err := c.Patch(ctx, c.appPath(appID)+"/authorized-apis/"+url.PathEscape(apiID), body, nil)
	return err
}

// RevokeAPI removes an API resource authorization from an application.
func (c *Client) RevokeAPI(ctx context.Context, appID, apiID string) error {
	return c.Delete(ctx, c.appPath(appID)+"/authorized-apis/"+url.PathEscape(apiID))
}

// ShareApplication shares an application with sub-organizations.
func (c *Client) ShareApplication(ctx context.Context, appID string, shareWithAll bool, orgIDs []string) error {
	body := map[string]any{"shareWithAllChildren": shareWithAll, "sharedOrganizations": nonNil(orgIDs)}
	_, err := c.Post(ctx, c.appPath(appID)+"/share", body, nil)
	return err
}

// SharedOrganization is an organization an application is shared with.
type SharedOrganization struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// GetApplicationSharedOrganizations lists the organizations an application is shared with.
func (c *Client) GetApplicationSharedOrganizations(ctx context.Context, appID string) ([]SharedOrganization, error) {
	var out struct {
		Organizations []SharedOrganization `json:"organizations"`
	}
	if err := c.Get(ctx, c.appPath(appID)+"/share", nil, &out); err != nil {
		return nil, err
	}
	return out.Organizations, nil
}

// UnshareApplication stops sharing an application with one organization.
func (c *Client) UnshareApplication(ctx context.Context, appID, orgID string) error {
	return c.Delete(ctx, c.appPath(appID)+"/share/"+url.PathEscape(orgID))
}

// UnshareApplicationAll stops sharing an application with every organization.
func (c *Client) UnshareApplicationAll(ctx context.Context, appID string) error {
	return c.Delete(ctx, c.appPath(appID)+"/shared-apps")
}

func nonNil[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}
