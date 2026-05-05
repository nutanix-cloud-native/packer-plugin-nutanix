package nutanix

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"sort"

	convergedv4 "github.com/nutanix-cloud-native/prism-go-client/converged/v4"
	"github.com/nutanix-cloud-native/prism-go-client/environment/types"
	v4 "github.com/nutanix-cloud-native/prism-go-client/v4"
)

// ntnxAPIKeyHeader is the HTTP header recognised by Prism Central for API key
// auth. The casing matches what prism-go-client emits (see v4/v4.go).
const ntnxAPIKeyHeader = "X-ntnx-api-key"

// v4SDKClientCache caches the underlying *v4.Client per connection. Session
// auth is enabled to match the previous behaviour.
var v4SDKClientCache = v4.NewClientCache(v4.WithSessionAuth(true))

// v4CacheParams implements types.CachedClientParams for the v4 SDK client cache.
type v4CacheParams struct {
	endpoint      string
	port          int32
	username      string
	password      string
	apiKey        string
	customHeaders map[string]string
	insecure      bool
}

// Key returns a unique cache key for this Prism Central connection. Includes
// API key and custom headers so different auth produces different cache
// entries; the values themselves are hashed to avoid leaking secrets into
// log lines that may print the key.
func (p *v4CacheParams) Key() string {
	h := sha256.New()
	h.Write([]byte(p.apiKey))
	h.Write([]byte{0})
	keys := make([]string, 0, len(p.customHeaders))
	for k := range p.customHeaders {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		h.Write([]byte(k))
		h.Write([]byte{0})
		h.Write([]byte(p.customHeaders[k]))
		h.Write([]byte{0})
	}
	return fmt.Sprintf("packer:%s:%d:%s", p.endpoint, p.port, hex.EncodeToString(h.Sum(nil))[:16])
}

// ManagementEndpoint returns the management endpoint for client creation and
// cache validation. When an API key is configured we use the prism-go-client
// trick of putting the api-key header name in Username and the key itself in
// Password — v4.setAuthHeader detects this and emits the X-ntnx-api-key
// header instead of Basic auth. The cache also requires both fields to be
// non-empty.
func (p *v4CacheParams) ManagementEndpoint() types.ManagementEndpoint {
	u := &url.URL{
		Scheme: "https",
		Host:   fmt.Sprintf("%s:%d", p.endpoint, p.port),
	}
	creds := types.ApiCredentials{
		Username: p.username,
		Password: p.password,
	}
	if p.apiKey != "" {
		creds = types.ApiCredentials{
			Username: ntnxAPIKeyHeader,
			Password: p.apiKey,
		}
	}
	return types.ManagementEndpoint{
		ApiCredentials: creds,
		Address:        u,
		Insecure:       p.insecure,
	}
}

// getV4ConvergedClient returns a converged v4 client for the given params,
// reusing the cached underlying *v4.Client and applying any custom headers
// to all SDK API instances. AddDefaultHeader is idempotent for a given key,
// so re-applying on cache hits is safe.
func getV4ConvergedClient(params *v4CacheParams, opts ...types.ClientOption[v4.Client]) (*convergedv4.Client, error) {
	v4Client, err := v4SDKClientCache.GetOrCreate(params, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to get or create V4 client: %w", err)
	}
	if len(params.customHeaders) > 0 {
		applyCustomHeaders(v4Client, params.customHeaders)
	}
	return convergedv4.NewClientFromV4SDKClient(v4Client), nil
}

// applyCustomHeaders sets every entry in headers as a default header on each
// underlying SDK ApiClient inside the v4.Client. The v4 client groups its
// API instances by service domain (vmm, networking, clustermgmt, prism,
// volumes, iam), and each group shares a single ApiClient — so we pick one
// instance per group rather than walking every field.
func applyCustomHeaders(c *v4.Client, headers map[string]string) {
	if c == nil || len(headers) == 0 {
		return
	}
	add := func(addHeader func(string, string)) {
		for k, v := range headers {
			addHeader(k, v)
		}
	}
	if c.VmApiInstance != nil && c.VmApiInstance.ApiClient != nil {
		add(c.VmApiInstance.ApiClient.AddDefaultHeader)
	}
	if c.SubnetsApiInstance != nil && c.SubnetsApiInstance.ApiClient != nil {
		add(c.SubnetsApiInstance.ApiClient.AddDefaultHeader)
	}
	if c.ClustersApiInstance != nil && c.ClustersApiInstance.ApiClient != nil {
		add(c.ClustersApiInstance.ApiClient.AddDefaultHeader)
	}
	if c.StorageContainerAPI != nil && c.StorageContainerAPI.ApiClient != nil {
		add(c.StorageContainerAPI.ApiClient.AddDefaultHeader)
	}
	if c.TasksApiInstance != nil && c.TasksApiInstance.ApiClient != nil {
		add(c.TasksApiInstance.ApiClient.AddDefaultHeader)
	}
	if c.VolumeGroupsApiInstance != nil && c.VolumeGroupsApiInstance.ApiClient != nil {
		add(c.VolumeGroupsApiInstance.ApiClient.AddDefaultHeader)
	}
	if c.UsersApiInstance != nil && c.UsersApiInstance.ApiClient != nil {
		add(c.UsersApiInstance.ApiClient.AddDefaultHeader)
	}
}
