package userstore_test

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/hayone1/terraform-provider-wso2is/internal/acctest"
)

// jdbc returns connection settings for a secondary JDBC user store. With
// WSO2IS_TEST_JDBC_URL unset, an embedded H2 database is used: the user store is created,
// but users cannot be added to it.
func jdbc() (url, driver, user, password string, functional bool) {
	if u := os.Getenv("WSO2IS_TEST_JDBC_URL"); u != "" {
		return u, os.Getenv("WSO2IS_TEST_JDBC_DRIVER"), os.Getenv("WSO2IS_TEST_JDBC_USERNAME"), os.Getenv("WSO2IS_TEST_JDBC_PASSWORD"), true
	}
	return "jdbc:h2:./repository/database/TFACC_USERSTORE;DB_CLOSE_ON_EXIT=FALSE", "org.h2.Driver", "wso2carbon", "wso2carbon", false
}

func TestAccUserstore_jdbc(t *testing.T) {
	domain := strings.ToUpper(strings.ReplaceAll(acctest.RandomName("us"), "-", ""))
	url, driver, user, pass, functional := jdbc()
	config := func(desc, maxUserList string, withUser bool) string {
		s := fmt.Sprintf(`
resource "wso2is_userstore" "test" {
  name        = %[1]q
  type        = "UniqueIDJDBCUserStoreManager"
  description = %[6]q
  properties = {
    url                   = %[2]q
    driverName            = %[3]q
    userName              = %[4]q
    MaxUserNameListLength = %[7]q
  }
  secret_properties = {
    password = %[5]q
  }
}
`, domain, url, driver, user, pass, desc, maxUserList)
		if withUser {
			s += fmt.Sprintf(`
resource "wso2is_user" "in_secondary" {
  username   = "%[1]suser"
  user_store = wso2is_userstore.test.name
  password   = "Secondary-Passw0rd!"
  email      = "%[1]suser@example.com"
}
`, strings.ToLower(domain))
		}
		return s
	}
	steps := []resource.TestStep{
		{
			Config: config("Customer accounts", "100", false),
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("wso2is_userstore.test", "name", domain),
				resource.TestCheckResourceAttr("wso2is_userstore.test", "enabled", "true"),
				resource.TestCheckResourceAttr("wso2is_userstore.test", "properties.driverName", driver),
				resource.TestCheckResourceAttrSet("wso2is_userstore.test", "type_id"),
			),
		},
		{
			Config: config("Customer accounts (EU)", "500", functional),
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("wso2is_userstore.test", "description", "Customer accounts (EU)"),
				resource.TestCheckResourceAttr("wso2is_userstore.test", "properties.MaxUserNameListLength", "500"),
			),
		},
		{
			ResourceName:            "wso2is_userstore.test",
			ImportState:             true,
			ImportStateId:           domain,
			ImportStateVerify:       true,
			ImportStateVerifyIgnore: []string{"properties", "secret_properties"},
		},
	}
	if functional {
		steps[1].Check = resource.ComposeAggregateTestCheckFunc(steps[1].Check,
			resource.TestCheckResourceAttr("wso2is_user.in_secondary", "user_store", domain),
		)
	}
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps:                    steps,
	})
}
