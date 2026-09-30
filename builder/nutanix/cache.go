package nutanix

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"sort"
	"sync"

	convergedv4 "github.com/nutanix-cloud-native/prism-go-client/converged/v4"
	"github.com/nutanix-cloud-native/prism-go-client/environment/types"
	v4 "github.com/nutanix-cloud-native/prism-go-client/v4"
)

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
	// transfer marks a client created with the transfer read timeout. It gets
	// its own cache entry because a cache hit ignores client options, so a
	// shared entry would keep whichever timeout was created first.
	transfer bool
	// objectsUpload makes a username/password client even when an API key is
	// set, for the Objects Lite image upload only. See ManagementEndpoint.
	objectsUpload bool
}

// Key returns a unique cache key for this Prism Central connection. Includes
// the custom headers and the transfer/upload flags so different headers or
// client options produce different cache entries; the header values are
// hashed to avoid leaking secrets into log lines that may print the key.
// Credentials, including the API key, are not part of the key: the cache's
// validation hash covers ManagementEndpoint, so a change of credentials
// replaces the cached client.
func (p *v4CacheParams) Key() string {
	h := sha256.New()
	if p.transfer {
		h.Write([]byte("transfer"))
	}
	h.Write([]byte{0})
	if p.objectsUpload {
		h.Write([]byte("objects-upload"))
	}
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
// cache validation. When an API key is configured, only the key is passed, so
// requests carry the X-ntnx-api-key header and no Basic auth.
//
// The exception is objectsUpload: the Objects Lite image upload signs its S3
// requests with the username/password held on the client, and the image create
// that follows runs on the same client. That client therefore uses
// username/password only, as a build without an API key does, rather than
// sending both identities. It is a separate cache entry used only for uploads.
func (p *v4CacheParams) ManagementEndpoint() types.ManagementEndpoint {
	u := &url.URL{
		Scheme: "https",
		Host:   fmt.Sprintf("%s:%d", p.endpoint, p.port),
	}
	creds := types.ApiCredentials{
		Username: p.username,
		Password: p.password,
	}
	if p.apiKey != "" && !p.objectsUpload {
		creds = types.ApiCredentials{APIKey: p.apiKey}
	}
	return types.ManagementEndpoint{
		ApiCredentials: creds,
		Address:        u,
		Insecure:       p.insecure,
	}
}

// customHeadersApplied records, per cached *v4.Client, that its custom headers
// have been applied. The SDK reads its default headers map on every request,
// so writing it again on a cache hit races with requests already using the
// client from another goroutine.
var customHeadersApplied sync.Map // *v4.Client -> *sync.Once

// getV4ConvergedClient returns a converged v4 client for the given params,
// reusing the cached underlying *v4.Client and applying any custom headers
// to all SDK API instances once per client. The cache key includes the
// headers, so a cached client always carries the same set.
func getV4ConvergedClient(params *v4CacheParams, opts ...types.ClientOption[v4.Client]) (*convergedv4.Client, error) {
	v4Client, err := v4SDKClientCache.GetOrCreate(params, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to get or create V4 client: %w", err)
	}
	if len(params.customHeaders) > 0 {
		once, _ := customHeadersApplied.LoadOrStore(v4Client, new(sync.Once))
		once.(*sync.Once).Do(func() { applyCustomHeaders(v4Client, params.customHeaders) })
	}
	return convergedv4.NewClientFromV4SDKClient(v4Client), nil
}

// applyCustomHeaders sets every entry in headers as a default header on the SDK
// ApiClients the plugin uses: vmm, networking, clustermgmt, prism (tasks), volumes
// and iam users. API instances within a group usually share one ApiClient, so
// setting the same one twice is harmless. Other ApiClients in the v4.Client (other iam clients,
// multidomain, datapolicies, monitoring) are not covered; add them here before
// calling them.
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
