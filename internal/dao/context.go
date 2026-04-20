package dao

import (
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
