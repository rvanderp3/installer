package clusterapi

// ExternalProvider is the generic CAPI lifecycle implementation used when
// controller binaries and manifests are supplied at runtime.
type ExternalProvider struct{}

var _ Provider = (*ExternalProvider)(nil)

// Name returns the generic external platform name.
func (*ExternalProvider) Name() string { return "external" }

// PublicGatherEndpoint returns the default endpoint used for runtime providers.
func (*ExternalProvider) PublicGatherEndpoint() GatherEndpoint { return InternalIP }
