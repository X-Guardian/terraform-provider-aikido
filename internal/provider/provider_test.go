// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
)

var testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"aikido": providerserver.NewProtocol6WithError(New("test")()),
}

func testAccPreCheck(t *testing.T) {
	if os.Getenv("AIKIDO_CLIENT_ID") == "" {
		t.Skip("AIKIDO_CLIENT_ID must be set for acceptance tests")
	}
	if os.Getenv("AIKIDO_CLIENT_SECRET") == "" {
		t.Skip("AIKIDO_CLIENT_SECRET must be set for acceptance tests")
	}
}
