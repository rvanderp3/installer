package clusterapi

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v2"
)

const (
	// RuntimeProviderConfigFile is the file containing runtime provider
	// definitions in a provider bundle.
	RuntimeProviderConfigFile = "providers.yaml"
)

// RuntimeProvider describes a CAPI controller supplied outside the installer
// binary. Binary and Components are resolved relative to the provider bundle.
type RuntimeProvider struct {
	Name       string            `yaml:"name"`
	Binary     string            `yaml:"binary"`
	Components []string          `yaml:"components"`
	Args       []string          `yaml:"args,omitempty"`
	Env        map[string]string `yaml:"env,omitempty"`
}

type runtimeProviderConfig struct {
	Providers []RuntimeProvider `yaml:"providers"`
}

// LoadRuntimeProviders loads and validates providers.yaml from a runtime
// provider bundle. Paths in the bundle are deliberately confined to the
// bundle directory so a configuration cannot cause the installer to execute
// an unrelated host binary or install unrelated CRDs.
func LoadRuntimeProviders(dir string) ([]RuntimeProvider, error) {
	if dir == "" {
		return nil, fmt.Errorf("runtime provider directory is empty")
	}

	root, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve runtime provider directory: %w", err)
	}

	data, err := os.ReadFile(filepath.Join(root, RuntimeProviderConfigFile))
	if err != nil {
		return nil, fmt.Errorf("failed to read runtime provider configuration: %w", err)
	}

	var config runtimeProviderConfig
	if err := yaml.UnmarshalStrict(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse runtime provider configuration: %w", err)
	}
	if len(config.Providers) == 0 {
		return nil, fmt.Errorf("runtime provider configuration contains no providers")
	}

	for i := range config.Providers {
		provider := &config.Providers[i]
		if provider.Name == "" {
			return nil, fmt.Errorf("runtime provider %d has no name", i)
		}
		if strings.ContainsAny(provider.Name, `/\\`) {
			return nil, fmt.Errorf("runtime provider %q has an invalid name", provider.Name)
		}
		if provider.Binary == "" {
			return nil, fmt.Errorf("runtime provider %q has no binary", provider.Name)
		}

		binary, err := bundlePath(root, provider.Binary)
		if err != nil {
			return nil, fmt.Errorf("runtime provider %q binary: %w", provider.Name, err)
		}
		info, err := os.Stat(binary)
		if err != nil {
			return nil, fmt.Errorf("runtime provider %q binary: %w", provider.Name, err)
		}
		if !info.Mode().IsRegular() || info.Mode()&0o111 == 0 {
			return nil, fmt.Errorf("runtime provider %q binary is not executable: %s", provider.Name, binary)
		}
		if err := verifyBundlePath(root, binary); err != nil {
			return nil, fmt.Errorf("runtime provider %q binary: %w", provider.Name, err)
		}
		provider.Binary = binary

		for j, component := range provider.Components {
			resolved, err := bundlePath(root, component)
			if err != nil {
				return nil, fmt.Errorf("runtime provider %q component %d: %w", provider.Name, j, err)
			}
			info, err := os.Stat(resolved)
			if err != nil {
				return nil, fmt.Errorf("runtime provider %q component %d: %w", provider.Name, j, err)
			}
			if !info.Mode().IsRegular() {
				return nil, fmt.Errorf("runtime provider %q component is not a regular file: %s", provider.Name, resolved)
			}
			if err := verifyBundlePath(root, resolved); err != nil {
				return nil, fmt.Errorf("runtime provider %q component %d: %w", provider.Name, j, err)
			}
			provider.Components[j] = resolved
		}
	}

	return config.Providers, nil
}

func verifyBundlePath(root, path string) error {
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return err
	}
	realPath, err := filepath.EvalSymlinks(path)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(realRoot, realPath)
	if err != nil {
		return err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("path escapes runtime provider directory")
	}
	return nil
}

func bundlePath(root, value string) (string, error) {
	path := value
	if !filepath.IsAbs(path) {
		path = filepath.Join(root, path)
	}
	path, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return "", err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path escapes runtime provider directory")
	}
	return path, nil
}
