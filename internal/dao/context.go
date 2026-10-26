package dao

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	dockerconfig "github.com/docker/cli/cli/config"
	dockercontext "github.com/docker/cli/cli/context/docker"
	"github.com/docker/cli/cli/context/store"
	dockerclient "github.com/docker/docker/client"
)

type ContextInfo struct {
	Name           string
	Description    string
	DockerEndpoint string
}

type dockerContextMeta struct {
	Description string `json:"Description,omitempty"`
}

func newContextStore() *store.ContextStore {
	return store.New(dockerconfig.ContextStoreDir(), store.NewConfig(
		func() interface{} {
			return &dockerContextMeta{}
		},
		store.EndpointTypeGetter(dockercontext.DockerEndpoint, func() interface{} { return &dockercontext.EndpointMeta{} }),
	))
}

func ListContexts() ([]ContextInfo, error) {
	contexts, err := newContextStore().List()
	if err != nil {
		return nil, err
	}
	contexts = append(contexts, defaultContextMetadata())

	items := make([]ContextInfo, 0, len(contexts))
	for _, raw := range contexts {
		meta := dockerContextMeta{}
		if typed, ok := raw.Metadata.(dockerContextMeta); ok {
			meta = typed
		}

		host := ""
		if endpoint, err := dockercontext.EndpointFromContext(raw); err == nil {
			host = endpoint.Host
		}

		items = append(items, ContextInfo{
			Name:           raw.Name,
			Description:    strings.TrimSpace(meta.Description),
			DockerEndpoint: strings.TrimSpace(host),
		})
	}

	sort.Slice(items, func(i, j int) bool {
		return strings.ToLower(items[i].Name) < strings.ToLower(items[j].Name)
	})

	return items, nil
}

func (d *DockerClient) ListContexts() ([]ContextInfo, error) {
	return ListContexts()
}

func (d *DockerClient) RenameContext(oldName, newName string) error {
	oldName = strings.TrimSpace(oldName)
	newName = strings.TrimSpace(newName)
	if oldName == newName {
		return nil
	}
	if err := store.ValidateContextName(newName); err != nil {
		return err
	}

	contextStore := newContextStore()
	contexts, err := contextStore.List()
	if err != nil {
		return err
	}
	for _, context := range contexts {
		if context.Name == newName {
			return fmt.Errorf("context %q already exists", newName)
		}
	}

	metadata, err := contextStore.GetMetadata(oldName)
	if err != nil {
		return fmt.Errorf("failed to load context %q: %w", oldName, err)
	}

	tlsFiles, err := contextStore.ListTLSFiles(oldName)
	if err != nil {
		return err
	}
	tlsData := &store.ContextTLSData{Endpoints: make(map[string]store.EndpointTLSData, len(tlsFiles))}
	for endpoint, files := range tlsFiles {
		endpointData := store.EndpointTLSData{Files: make(map[string][]byte, len(files))}
		for _, file := range files {
			data, err := contextStore.GetTLSData(oldName, endpoint, file)
			if err != nil {
				return err
			}
			endpointData.Files[file] = data
		}
		tlsData.Endpoints[endpoint] = endpointData
	}

	metadata.Name = newName
	if err := contextStore.CreateOrUpdate(metadata); err != nil {
		return fmt.Errorf("failed to create renamed context: %w", err)
	}
	cleanupNew := func() {
		_ = contextStore.Remove(newName)
	}
	if err := contextStore.ResetTLSMaterial(newName, tlsData); err != nil {
		cleanupNew()
		return fmt.Errorf("failed to copy context TLS data: %w", err)
	}

	configFile := dockerconfig.LoadDefaultConfigFile(io.Discard)
	wasDockerDefault := configFile.CurrentContext == oldName
	if wasDockerDefault {
		configFile.CurrentContext = newName
		if err := configFile.Save(); err != nil {
			cleanupNew()
			return fmt.Errorf("failed to update Docker's current context: %w", err)
		}
	}

	if err := contextStore.Remove(oldName); err != nil {
		if wasDockerDefault {
			configFile.CurrentContext = oldName
			_ = configFile.Save()
		}
		cleanupNew()
		return err
	}
	return nil
}

func (d *DockerClient) UpdateAndRenameContext(oldName, newName, description, host string) error {
	renamed := strings.TrimSpace(oldName) != strings.TrimSpace(newName)
	if renamed {
		if err := d.RenameContext(oldName, newName); err != nil {
			return err
		}
	}

	if err := d.UpdateContext(newName, description, host); err != nil {
		if renamed {
			if rollbackErr := d.RenameContext(newName, oldName); rollbackErr != nil {
				return fmt.Errorf("%w; additionally failed to restore context name: %v", err, rollbackErr)
			}
		}
		return err
	}
	return nil
}

func defaultContextMetadata() store.Metadata {
	host := os.Getenv("DOCKER_HOST")
	if host == "" {
		host = dockerclient.DefaultDockerHost
	}

	return store.Metadata{
		Name: "default",
		Metadata: dockerContextMeta{
			Description: "Current DOCKER_HOST based configuration",
		},
		Endpoints: map[string]any{
			dockercontext.DockerEndpoint: dockercontext.EndpointMeta{Host: host},
		},
	}
}
