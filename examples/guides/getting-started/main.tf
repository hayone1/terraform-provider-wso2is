terraform {
  required_providers {
    wso2is = {
      source = "hayone1/wso2is"
    }
  }
}

# Connect to the local server that you started with:
#   docker run -d --name wso2is -p 9443:9443 wso2/wso2is:7.3.0
provider "wso2is" {
  base_url = "https://localhost:9443"
  username = "admin"
  password = "admin"
  # The local server uses a self-signed certificate. Only skip the check on a test server.
  insecure_skip_verify = true
}

# The API that the application calls. Each scope is one permission in the API.
resource "wso2is_api_resource" "notes" {
  name       = "Notes API"
  identifier = "https://api.example.com/notes"
  scopes = [
    { name = "notes:read", display_name = "Read notes" },
    { name = "notes:write", display_name = "Write notes" },
  ]
}

# A single-page application that signs users in with OIDC.
resource "wso2is_application" "notes_spa" {
  name        = "Notes"
  template_id = "single-page-application"
  oidc = {
    grant_types     = ["authorization_code", "refresh_token"]
    public_client   = true
    callback_urls   = ["http://localhost:3000"]
    allowed_origins = ["http://localhost:3000"]
    pkce = {
      mandatory = true
    }
  }
  # The application uses its own roles, such as "editor" below.
  associated_roles = {
    allowed_audience = "APPLICATION"
  }
}

# Allow the application to request both scopes of the API.
resource "wso2is_application_authorized_api" "notes" {
  application_id  = wso2is_application.notes_spa.id
  api_resource_id = wso2is_api_resource.notes.id
  scopes          = ["notes:read", "notes:write"]
}

# An application role that grants both scopes. A role can only grant scopes that the
# application may request, so the role waits for the authorization above.
resource "wso2is_role" "editor" {
  name          = "editor"
  audience_type = "application"
  audience_id   = wso2is_application.notes_spa.id
  permissions   = ["notes:read", "notes:write"]
  depends_on    = [wso2is_application_authorized_api.notes]
}

# A group whose members get the editor role.
resource "wso2is_group" "editors" {
  name = "editors"
}

resource "wso2is_role_assignment" "editors" {
  role_id   = wso2is_role.editor.id
  group_ids = [wso2is_group.editors.id]
}

# A user who can sign in to the application.
resource "wso2is_user" "alice" {
  username    = "alice"
  given_name  = "Alice"
  family_name = "Smith"
  email       = "alice@example.com"
  password    = "Alice-Passw0rd!"
}

# Add Alice to the editors group.
resource "wso2is_group_members" "editors" {
  group_id = wso2is_group.editors.id
  user_ids = [wso2is_user.alice.id]
}

# Use this client ID in your application's OIDC settings.
output "client_id" {
  value = wso2is_application.notes_spa.oidc.client_id
}
