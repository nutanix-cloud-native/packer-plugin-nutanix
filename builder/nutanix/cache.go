package nutanix

import (
	"fmt"
	"net/url"

	convergedv4 "github.com/nutanix-cloud-native/prism-go-client/converged/v4"
	"github.com/nutanix-cloud-native/prism-go-client/environment/types"
	v4 "github.com/nutanix-cloud-native/prism-go-client/v4"
)

// convergedV4ClientCache is the shared cache for V4 converged clients (session auth enabled).
var convergedV4ClientCache = convergedv4.NewClientCache(v4.WithSessionAuth(true))

// v4SDKClientCache is the shared cache for raw V4 SDK clients (session auth enabled).
var v4SDKClientCache = v4.NewClientCache(v4.WithSessionAuth(true))

// v4CacheParams implements types.CachedClientParams for use with both the converged V4 and raw V4 SDK client caches.
type v4CacheParams struct {
	endpoint string
	port     int32
	username string
	password string
	insecure bool
}

// Key returns a unique cache key for this Prism Central connection.
func (p *v4CacheParams) Key() string {
	return fmt.Sprintf("packer:%s:%d", p.endpoint, p.port)
}

// ManagementEndpoint returns the management endpoint for client creation and cache validation.
func (p *v4CacheParams) ManagementEndpoint() types.ManagementEndpoint {
	u := &url.URL{
		Scheme: "https",
		Host:   fmt.Sprintf("%s:%d", p.endpoint, p.port),
	}
	return types.ManagementEndpoint{
		ApiCredentials: types.ApiCredentials{
			Username: p.username,
			Password: p.password,
		},
		Address:  u,
		Insecure: p.insecure,
	}
}
