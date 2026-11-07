package provider

import (
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/resource"

	"github.com/hayone1/terraform-provider-wso2is/internal/service/action"
	"github.com/hayone1/terraform-provider-wso2is/internal/service/apiresource"
	"github.com/hayone1/terraform-provider-wso2is/internal/service/application"
	"github.com/hayone1/terraform-provider-wso2is/internal/service/authenticator"
	"github.com/hayone1/terraform-provider-wso2is/internal/service/branding"
	"github.com/hayone1/terraform-provider-wso2is/internal/service/claim"
	"github.com/hayone1/terraform-provider-wso2is/internal/service/config"
	"github.com/hayone1/terraform-provider-wso2is/internal/service/flow"
	"github.com/hayone1/terraform-provider-wso2is/internal/service/governance"
	"github.com/hayone1/terraform-provider-wso2is/internal/service/identity"
	"github.com/hayone1/terraform-provider-wso2is/internal/service/idp"
	"github.com/hayone1/terraform-provider-wso2is/internal/service/idv"
	"github.com/hayone1/terraform-provider-wso2is/internal/service/lookup"
	"github.com/hayone1/terraform-provider-wso2is/internal/service/notification"
	"github.com/hayone1/terraform-provider-wso2is/internal/service/organization"
	"github.com/hayone1/terraform-provider-wso2is/internal/service/scripting"
	"github.com/hayone1/terraform-provider-wso2is/internal/service/tenant"
	"github.com/hayone1/terraform-provider-wso2is/internal/service/userstore"
	"github.com/hayone1/terraform-provider-wso2is/internal/service/vc"
	"github.com/hayone1/terraform-provider-wso2is/internal/service/webhook"
	"github.com/hayone1/terraform-provider-wso2is/internal/service/workflow"
)

func allResources() []func() resource.Resource {
	return append(config.Resources(),
		application.NewResource,
		application.NewAuthorizedAPIResource,
		apiresource.NewResource,
		claim.NewClaimResource,
		claim.NewDialectResource,
		claim.NewExternalClaimResource,
		claim.NewOIDCScopeResource,
		idp.NewResource,
		governance.NewConnectorResource,
		governance.NewValidationRulesResource,
		userstore.NewResource,
		tenant.NewResource,
		branding.NewPreferenceResource,
		organization.NewResource,
		action.NewResource,
		scripting.NewLibraryResource,
		flow.NewResource,
		webhook.NewResource,
		workflow.NewResource,
		authenticator.NewCustomResource,
		idv.NewResource,
		vc.NewTemplateResource,
		workflow.NewAssociationResource,
		scripting.NewSecretResource,
		organization.NewShareResource,
		organization.NewDiscoveryConfigResource,
		organization.NewDiscoveryResource,
		organization.NewUserShareResource,
		branding.NewTextResource,
		notification.NewEmailSenderResource,
		notification.NewSMSSenderResource,
		notification.NewPushSenderResource,
		notification.NewTemplateResource,
		notification.NewTemplateTypeResource,
		identity.NewUserResource,
		identity.NewGroupResource,
		identity.NewGroupMembersResource,
		identity.NewRoleResource,
		identity.NewRoleAssignmentResource,
	)
}

func allDataSources() []func() datasource.DataSource {
	return []func() datasource.DataSource{
		apiresource.NewDataSource,
		governance.NewConnectorDataSource,
		organization.NewDataSource,
		flow.NewDataSource,
		authenticator.NewListDataSource,
		lookup.NewApplicationDataSource,
		lookup.NewIdentityProviderDataSource,
		lookup.NewUserDataSource,
		lookup.NewGroupDataSource,
		lookup.NewRoleDataSource,
		lookup.NewClaimDataSource,
		idp.NewMetadataDataSource,
	}
}
