package clusterapi

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoadRuntimeProviders(t *testing.T) {
	dir := t.TempDir()
	binary := filepath.Join(dir, "cluster-api-provider-proxmox")
	components := filepath.Join(dir, "proxmox-components.yaml")
	assert.NoError(t, os.WriteFile(binary, []byte("binary"), 0o700))
	assert.NoError(t, os.WriteFile(components, []byte("apiVersion: v1\nkind: ConfigMap\n"), 0o600))
	assert.NoError(t, os.WriteFile(filepath.Join(dir, RuntimeProviderConfigFile), []byte(`providers:
- name: proxmox
  binary: cluster-api-provider-proxmox
  components:
  - proxmox-components.yaml
  args:
  - --health-probe-bind-address=127.0.0.1:0
  env:
    PROXMOX_INSECURE: "true"
`), 0o600))

	providers, err := LoadRuntimeProviders(dir)
	assert.NoError(t, err)
	if assert.Len(t, providers, 1) {
		assert.Equal(t, "proxmox", providers[0].Name)
		assert.Equal(t, binary, providers[0].Binary)
		assert.Equal(t, []string{components}, providers[0].Components)
		assert.Equal(t, []string{"--health-probe-bind-address=127.0.0.1:0"}, providers[0].Args)
		assert.Equal(t, "true", providers[0].Env["PROXMOX_INSECURE"])
	}
}

func TestLoadRuntimeProvidersRejectsPathsOutsideBundle(t *testing.T) {
	dir := t.TempDir()
	binary := filepath.Join(dir, "cluster-api-provider-proxmox")
	assert.NoError(t, os.WriteFile(binary, []byte("binary"), 0o700))
	assert.NoError(t, os.WriteFile(filepath.Join(dir, RuntimeProviderConfigFile), []byte(`providers:
- name: proxmox
  binary: ../cluster-api-provider-proxmox
`), 0o600))

	_, err := LoadRuntimeProviders(dir)
	assert.EqualError(t, err, `runtime provider "proxmox" binary: path escapes runtime provider directory`)
}

func TestLoadRuntimeProvidersRejectsSymlinkedBinary(t *testing.T) {
	dir := t.TempDir()
	outside := filepath.Join(t.TempDir(), "provider")
	assert.NoError(t, os.WriteFile(outside, []byte("binary"), 0o700))
	assert.NoError(t, os.Symlink(outside, filepath.Join(dir, "provider")))
	assert.NoError(t, os.WriteFile(filepath.Join(dir, RuntimeProviderConfigFile), []byte(`providers:
- name: proxmox
  binary: provider
`), 0o600))

	_, err := LoadRuntimeProviders(dir)
	assert.EqualError(t, err, `runtime provider "proxmox" binary: path escapes runtime provider directory`)
}

func TestLoadRuntimeProvidersRequiresExecutable(t *testing.T) {
	dir := t.TempDir()
	binary := filepath.Join(dir, "cluster-api-provider-proxmox")
	assert.NoError(t, os.WriteFile(binary, []byte("binary"), 0o600))
	assert.NoError(t, os.WriteFile(filepath.Join(dir, RuntimeProviderConfigFile), []byte(`providers:
- name: proxmox
  binary: cluster-api-provider-proxmox
`), 0o600))

	_, err := LoadRuntimeProviders(dir)
	assert.EqualError(t, err, `runtime provider "proxmox" binary is not executable: `+binary)
}
