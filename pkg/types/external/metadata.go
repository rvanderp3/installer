package external

// Metadata contains the external platform information needed after the
// install-config is no longer available.
type Metadata struct {
	// PlatformName identifies the external infrastructure provider.
	PlatformName string `json:"platformName,omitempty"`
}
