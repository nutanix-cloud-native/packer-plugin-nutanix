module github.com/nutanix-cloud-native/packer-plugin-nutanix

go 1.25.0

require (
	github.com/hashicorp/hcl/v2 v2.24.0
	github.com/hashicorp/packer-plugin-sdk v0.6.7
	github.com/mitchellh/go-vnc v0.0.0-20150629162542-723ed9867aed
	github.com/nutanix-cloud-native/prism-go-client v0.7.1
	github.com/nutanix/ntnx-api-golang-clients/clustermgmt-go-client/v4 v4.2.2
	github.com/nutanix/ntnx-api-golang-clients/networking-go-client/v4 v4.3.1
	github.com/nutanix/ntnx-api-golang-clients/vmm-go-client/v4 v4.2.2
	github.com/zclconf/go-cty v1.16.3
	golang.org/x/net v0.52.0
)

require (
	cel.dev/expr v0.25.1 // indirect
	cloud.google.com/go v0.123.0 // indirect
	cloud.google.com/go/auth v0.18.2 // indirect
	cloud.google.com/go/auth/oauth2adapt v0.2.8 // indirect
	cloud.google.com/go/compute/metadata v0.9.0 // indirect
	cloud.google.com/go/iam v1.5.3 // indirect
	cloud.google.com/go/monitoring v1.24.3 // indirect
	cloud.google.com/go/storage v1.61.3 // indirect
	github.com/Azure/go-ntlmssp v0.0.0-20221128193559-754e69321358 // indirect
	github.com/ChrisTrenkamp/goxpath v0.0.0-20210404020558-97928f7e12b6 // indirect
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/detectors/gcp v1.30.0 // indirect
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/metric v0.55.0 // indirect
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/internal/resourcemapping v0.55.0 // indirect
	github.com/Microsoft/go-winio v0.6.2 // indirect
	github.com/PaesslerAG/gval v1.0.0 // indirect
	github.com/PaesslerAG/jsonpath v0.1.1 // indirect
	github.com/agext/levenshtein v1.2.3 // indirect
	github.com/apparentlymart/go-textseg/v13 v13.0.0 // indirect
	github.com/apparentlymart/go-textseg/v15 v15.0.0 // indirect
	github.com/armon/go-metrics v0.4.1 // indirect
	github.com/aws/aws-sdk-go v1.55.8 // indirect
	github.com/aws/aws-sdk-go-v2 v1.41.4 // indirect
	github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream v1.7.8 // indirect
	github.com/aws/aws-sdk-go-v2/config v1.32.12 // indirect
	github.com/aws/aws-sdk-go-v2/credentials v1.19.12 // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.18.20 // indirect
	github.com/aws/aws-sdk-go-v2/feature/s3/manager v1.22.9 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.4.20 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.7.20 // indirect
	github.com/aws/aws-sdk-go-v2/internal/ini v1.8.6 // indirect
	github.com/aws/aws-sdk-go-v2/internal/v4a v1.4.21 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.13.7 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/checksum v1.9.12 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.13.20 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/s3shared v1.19.20 // indirect
	github.com/aws/aws-sdk-go-v2/service/s3 v1.97.2 // indirect
	github.com/aws/aws-sdk-go-v2/service/secretsmanager v1.37.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/signin v1.0.8 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.30.13 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.35.17 // indirect
	github.com/aws/aws-sdk-go-v2/service/sts v1.41.9 // indirect
	github.com/aws/smithy-go v1.24.2 // indirect
	github.com/bgentry/go-netrc v0.0.0-20140422174119-9fd32a8b3d3d // indirect
	github.com/bodgit/ntlmssp v0.0.0-20240506230425-31973bb52d9b // indirect
	github.com/bodgit/windows v1.0.1 // indirect
	github.com/cenkalti/backoff/v3 v3.2.2 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/cncf/xds/go v0.0.0-20251210132809-ee656c7534f5 // indirect
	github.com/dylanmei/iso8601 v0.1.0 // indirect
	github.com/envoyproxy/go-control-plane/envoy v1.36.0 // indirect
	github.com/envoyproxy/protoc-gen-validate v1.3.0 // indirect
	github.com/fatih/color v1.16.0 // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/go-jose/go-jose/v4 v4.1.3 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-logr/zapr v1.3.0 // indirect
	github.com/go-openapi/analysis v0.24.3 // indirect
	github.com/go-openapi/errors v0.22.7 // indirect
	github.com/go-openapi/jsonpointer v0.22.5 // indirect
	github.com/go-openapi/jsonreference v0.21.5 // indirect
	github.com/go-openapi/loads v0.23.3 // indirect
	github.com/go-openapi/spec v0.22.4 // indirect
	github.com/go-openapi/strfmt v0.26.1 // indirect
	github.com/go-openapi/swag v0.25.5 // indirect
	github.com/go-openapi/swag/cmdutils v0.25.5 // indirect
	github.com/go-openapi/swag/conv v0.25.5 // indirect
	github.com/go-openapi/swag/fileutils v0.25.5 // indirect
	github.com/go-openapi/swag/jsonname v0.25.5 // indirect
	github.com/go-openapi/swag/jsonutils v0.25.5 // indirect
	github.com/go-openapi/swag/loading v0.25.5 // indirect
	github.com/go-openapi/swag/mangling v0.25.5 // indirect
	github.com/go-openapi/swag/netutils v0.25.5 // indirect
	github.com/go-openapi/swag/stringutils v0.25.5 // indirect
	github.com/go-openapi/swag/typeutils v0.25.5 // indirect
	github.com/go-openapi/swag/yamlutils v0.25.5 // indirect
	github.com/go-openapi/validate v0.25.2 // indirect
	github.com/go-viper/mapstructure/v2 v2.5.0 // indirect
	github.com/gofrs/flock v0.13.0 // indirect
	github.com/gofrs/uuid v4.4.0+incompatible // indirect
	github.com/google/s2a-go v0.1.9 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/googleapis/enterprise-certificate-proxy v0.3.14 // indirect
	github.com/googleapis/gax-go/v2 v2.18.0 // indirect
	github.com/hashicorp/consul/api v1.25.1 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-cleanhttp v0.5.2 // indirect
	github.com/hashicorp/go-getter/gcs/v2 v2.2.3 // indirect
	github.com/hashicorp/go-getter/s3/v2 v2.2.3 // indirect
	github.com/hashicorp/go-getter/v2 v2.2.3 // indirect
	github.com/hashicorp/go-hclog v1.6.3 // indirect
	github.com/hashicorp/go-immutable-radix v1.3.1 // indirect
	github.com/hashicorp/go-multierror v1.1.1 // indirect
	github.com/hashicorp/go-retryablehttp v0.7.7 // indirect
	github.com/hashicorp/go-rootcerts v1.0.2 // indirect
	github.com/hashicorp/go-safetemp v1.0.0 // indirect
	github.com/hashicorp/go-secure-stdlib/parseutil v0.1.6 // indirect
	github.com/hashicorp/go-secure-stdlib/strutil v0.1.2 // indirect
	github.com/hashicorp/go-sockaddr v1.0.7 // indirect
	github.com/hashicorp/go-uuid v1.0.3 // indirect
	github.com/hashicorp/go-version v1.8.0 // indirect
	github.com/hashicorp/golang-lru v0.5.4 // indirect
	github.com/hashicorp/hcl v1.0.0 // indirect
	github.com/hashicorp/serf v0.10.1 // indirect
	github.com/hashicorp/vault/api v1.14.0 // indirect
	github.com/hashicorp/yamux v0.1.2 // indirect
	github.com/jcmturner/aescts/v2 v2.0.0 // indirect
	github.com/jcmturner/dnsutils/v2 v2.0.0 // indirect
	github.com/jcmturner/gofork v1.7.6 // indirect
	github.com/jcmturner/goidentity/v6 v6.0.1 // indirect
	github.com/jcmturner/gokrb5/v8 v8.4.4 // indirect
	github.com/jcmturner/rpc/v2 v2.0.3 // indirect
	github.com/jehiah/go-strftime v0.0.0-20171201141054-1d33003b3869 // indirect
	github.com/jmespath/go-jmespath v0.4.0 // indirect
	github.com/klauspost/compress v1.18.5 // indirect
	github.com/kr/fs v0.1.0 // indirect
	github.com/masterzen/simplexml v0.0.0-20190410153822-31eea3082786 // indirect
	github.com/masterzen/winrm v0.0.0-20250927112105-5f8e6c707321 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/mitchellh/go-fs v0.0.0-20180402235330-b7b9ca407fff // indirect
	github.com/mitchellh/go-homedir v1.1.0 // indirect
	github.com/mitchellh/go-testing-interface v1.14.1 // indirect
	github.com/mitchellh/go-wordwrap v1.0.1 // indirect
	github.com/mitchellh/iochan v1.0.0 // indirect
	github.com/mitchellh/mapstructure v1.5.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/nu7hatch/gouuid v0.0.0-20131221200532-179d4d0c4d8d // indirect
	github.com/nutanix/ntnx-api-golang-clients/iam-go-client/v4 v4.0.1 // indirect
	github.com/nutanix/ntnx-api-golang-clients/prism-go-client/v4 v4.2.1 // indirect
	github.com/nutanix/ntnx-api-golang-clients/volumes-go-client/v4 v4.2.1 // indirect
	github.com/oklog/ulid/v2 v2.1.1 // indirect
	github.com/packer-community/winrmcp v0.0.0-20221126162354-6e900dd2c68f // indirect
	github.com/pkg/sftp v1.13.10 // indirect
	github.com/planetscale/vtprotobuf v0.6.1-0.20240319094008-0393e58bdf10 // indirect
	github.com/ryanuber/go-glob v1.0.0 // indirect
	github.com/sirupsen/logrus v1.9.3 // indirect
	github.com/spiffe/go-spiffe/v2 v2.6.0 // indirect
	github.com/tidwall/transform v0.0.0-20201103190739-32f242e2dbde // indirect
	github.com/ugorji/go/codec v1.3.1 // indirect
	github.com/ulikunitz/xz v0.5.15 // indirect
	github.com/vmihailenco/msgpack/v5 v5.3.5 // indirect
	github.com/vmihailenco/tagparser/v2 v2.0.0 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/contrib/detectors/gcp v1.39.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.63.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.61.0 // indirect
	go.opentelemetry.io/otel v1.42.0 // indirect
	go.opentelemetry.io/otel/metric v1.42.0 // indirect
	go.opentelemetry.io/otel/sdk v1.42.0 // indirect
	go.opentelemetry.io/otel/sdk/metric v1.42.0 // indirect
	go.opentelemetry.io/otel/trace v1.42.0 // indirect
	go.uber.org/multierr v1.10.0 // indirect
	go.uber.org/zap v1.27.0 // indirect
	go.yaml.in/yaml/v3 v3.0.4 // indirect
	golang.org/x/crypto v0.49.0 // indirect
	golang.org/x/exp v0.0.0-20230321023759-10a507213a29 // indirect
	golang.org/x/mobile v0.0.0-20260312152759-81488f6aeb60 // indirect
	golang.org/x/mod v0.34.0 // indirect
	golang.org/x/oauth2 v0.36.0 // indirect
	golang.org/x/sync v0.20.0 // indirect
	golang.org/x/sys v0.42.0 // indirect
	golang.org/x/term v0.41.0 // indirect
	golang.org/x/text v0.35.0 // indirect
	golang.org/x/time v0.15.0 // indirect
	golang.org/x/tools v0.43.0 // indirect
	google.golang.org/api v0.272.0 // indirect
	google.golang.org/genproto v0.0.0-20260217215200-42d3e9bedb6d // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20260217215200-42d3e9bedb6d // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260311181403-84a4fc48630c // indirect
	google.golang.org/grpc v1.79.3 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
	k8s.io/utils v0.0.0-20260319190234-28399d86e0b5 // indirect
)

replace (
	github.com/hashicorp/consul => github.com/hashicorp/consul v1.10.4
	github.com/hashicorp/vault => github.com/hashicorp/vault v1.4.2
)

replace github.com/zclconf/go-cty => github.com/nywilken/go-cty v1.13.3 // added by packer-sdc fix as noted in github.com/hashicorp/packer-plugin-sdk/issues/187
