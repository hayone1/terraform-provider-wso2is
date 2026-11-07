package action_test

import (
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/hayone1/terraform-provider-wso2is/internal/acctest"
)

func importID(address, typ string) func(*terraform.State) (string, error) {
	return func(s *terraform.State) (string, error) {
		id, err := acctest.ResourceID(s, address)
		return typ + "/" + id, err
	}
}
