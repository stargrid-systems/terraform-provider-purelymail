package provider

import (
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/stargrid-systems/terraform-provider-purelymail/internal/api"
	"github.com/stargrid-systems/terraform-provider-purelymail/internal/api/mock"
)

func TestAccDomainResource(t *testing.T) {
	// Create mock server using generated ServerInterface
	mockServer := mock.NewServer()
	handler := api.Handler(mockServer)
	ts := httptest.NewServer(handler)
	defer ts.Close()

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccDomainResourceConfig(ts.URL, "example.com", false, false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("purelymail_domain.test", "name", "example.com"),
					resource.TestCheckResourceAttr("purelymail_domain.test", "allow_account_reset", "false"),
					resource.TestCheckResourceAttr("purelymail_domain.test", "symbolic_subaddressing", "false"),
					resource.TestCheckResourceAttr("purelymail_domain.test", "is_shared", "false"),
					resource.TestCheckResourceAttr("purelymail_domain.test", "dns_summary.passes_mx", "true"),
					resource.TestCheckResourceAttr("purelymail_domain.test", "dns_summary.passes_spf", "true"),
					resource.TestCheckResourceAttr("purelymail_domain.test", "dns_summary.passes_dkim", "false"),
					resource.TestCheckResourceAttr("purelymail_domain.test", "dns_summary.passes_dmarc", "false"),
				),
			},
			// ImportState testing
			{
				ResourceName:            "purelymail_domain.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateId:           "example.com",
				ImportStateVerifyIgnore: []string{"recheck_dns"},
			},
			// Update and Read testing
			{
				Config: testAccDomainResourceConfig(ts.URL, "example.com", true, true),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("purelymail_domain.test", "allow_account_reset", "true"),
					resource.TestCheckResourceAttr("purelymail_domain.test", "symbolic_subaddressing", "true"),
				),
			},
			// Delete testing automatically occurs in TestCase
		},
	})
}

func testAccDomainResourceConfig(endpoint string, domainName string, allowAccountReset bool, symbolicSubaddressing bool) string {
	return fmt.Sprintf(`
provider "purelymail" {
	endpoint  = %[1]q
	api_token = "test-token"
}

resource "purelymail_domain" "test" {
	name                     = %[2]q
	allow_account_reset      = %[3]t
	symbolic_subaddressing   = %[4]t
}
`, endpoint, domainName, allowAccountReset, symbolicSubaddressing)
}
