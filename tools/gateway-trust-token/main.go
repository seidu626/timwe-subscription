// gateway-trust-token computes the X-Gateway-Trust header value for a given
// GATEWAY_TRUST_SECRET. Run this to get the static token to embed in krakend.json.
//
// Usage: GATEWAY_TRUST_SECRET=your-secret go run ./tools/gateway-trust-token/main.go
//
// Then replace __REPLACE_WITH_GATEWAY_TRUST_TOKEN__ in krakend/krakend.json with the output.
package main

import (
	"fmt"
	"os"

	"github.com/seidu626/subscription-manager/common/auth/tenantctx"
)

func main() {
	secret := os.Getenv("GATEWAY_TRUST_SECRET")
	if secret == "" && len(os.Args) > 1 {
		secret = os.Args[1]
	}
	if secret == "" {
		fmt.Fprintln(os.Stderr, "usage: GATEWAY_TRUST_SECRET=<secret> go run ./tools/gateway-trust-token/main.go")
		os.Exit(1)
	}
	fmt.Println(tenantctx.GatewayTrustToken(secret))
}
