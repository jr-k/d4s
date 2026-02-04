package dao

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/docker/cli/cli/config"
	clicontext "github.com/docker/cli/cli/context"
	"github.com/docker/cli/cli/context/docker"
	"github.com/docker/cli/cli/context/store"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/client"
	"github.com/jr-k/d4s/internal/dao/common"
	"github.com/jr-k/d4s/internal/dao/compose"
	"github.com/jr-k/d4s/internal/dao/docker/container"
	"github.com/jr-k/d4s/internal/dao/docker/image"
	"github.com/jr-k/d4s/internal/dao/docker/network"
	"github.com/jr-k/d4s/internal/dao/docker/secret"
	"github.com/jr-k/d4s/internal/dao/docker/volume"
	"github.com/jr-k/d4s/internal/dao/swarm/node"
	"github.com/jr-k/d4s/internal/dao/swarm/service"
)

// Re-export types for backward compatibility / convenience
type Resource = common.Resource
type HostStats = common.HostStats
type Container = container.Container
type Image = image.Image
type Volume = volume.Volume
type Network = network.Network
type Service = service.Service
type Node = node.Node
type Secret = secret.Secret
type ComposeProject = compose.ComposeProject

type DockerClient struct {
	Cli *client.Client
	Ctx context.Context
	ContextName string
	
	// Managers
	Container *container.Manager
	Image     *image.Manager
	Volume    *volume.Manager
	Network   *network.Manager
	Service   *service.Manager
	Node      *node.Manager
	Secret    *secret.Manager
	Compose   *compose.Manager
}

func NewDockerClient(contextName string) (*DockerClient, error) {
	logger, cleanup := initLogger()
	defer cleanup()

	ctxName, opts, err := resolveClientOpts(contextName, logger)
	if err != nil {
		return nil, err
	}

	cli, err := client.NewClientWithOpts(opts...)
	if err != nil {
		return nil, err
	}
	ctx := context.Background()
	
	return &DockerClient{
		Cli:         cli,
		Ctx:         ctx,
		ContextName: ctxName,
		Container:   container.NewManager(cli, ctx),
		Image:       image.NewManager(cli, ctx),
		Volume:      volume.NewManager(cli, ctx),
		Network:     network.NewManager(cli, ctx),
		Service:     service.NewManager(cli, ctx),
		Node:        node.NewManager(cli, ctx),
		Secret:      secret.NewManager(cli, ctx),
		Compose:     compose.NewManager(cli, ctx),
	}, nil
}

func initLogger() (*log.Logger, func()) {
	f, err := os.OpenFile("/tmp/d4s_debug_dao.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return log.New(io.Discard, "", 0), func() {}
	}
	return log.New(f, "d4s-dao: ", log.LstdFlags), func() { f.Close() }
}

func resolveClientOpts(flagContext string, logger *log.Logger) (string, []client.Opt, error) {
	opts := []client.Opt{
		client.WithAPIVersionNegotiation(),
	}

	// 1. Flag takes precedence
	if flagContext != "" {
		logger.Printf("Explicit context requested via flag: %s", flagContext)
		opts, err := loadSpecificContext(flagContext, logger, opts)
		return flagContext, opts, err
	}

	// 2. DOCKER_HOST takes precedence if no flag
	if h := os.Getenv("DOCKER_HOST"); h != "" {
		logger.Printf("DOCKER_HOST set to %s, using FromEnv", h)
		opts = append(opts, client.FromEnv)
		return "env", opts, nil
	}

	// 3. Identify Target Context
	targetCtx := "default"
	if envCtx := os.Getenv("DOCKER_CONTEXT"); envCtx != "" {
		targetCtx = envCtx
		logger.Printf("DOCKER_CONTEXT set to %s", targetCtx)
	} else {
		if cfg, err := config.Load(config.Dir()); err == nil && cfg.CurrentContext != "" {
			targetCtx = cfg.CurrentContext
			logger.Printf("Loaded CurrentContext from config: %s", targetCtx)
		} else if err != nil {
			logger.Printf("Failed to load config: %v", err)
		}
	}

	if targetCtx == "default" {
		logger.Println("Context is default, using FromEnv")
		opts = append(opts, client.FromEnv)
		return "default", opts, nil
	}

	// 4. Load Specific Context
	opts, err := loadSpecificContext(targetCtx, logger, opts)
	return targetCtx, opts, err
}

func loadSpecificContext(targetCtx string, logger *log.Logger, baseOpts []client.Opt) ([]client.Opt, error) {
	logger.Printf("Loading context: %s", targetCtx)
	
	// Create store with docker endpoint registered
	s := store.New(config.ContextStoreDir(), store.NewConfig(
		func() interface{} {
			return &map[string]interface{}{}
		},
		store.EndpointTypeGetter(docker.DockerEndpoint, func() interface{} { return &docker.EndpointMeta{} }),
	))

	meta, err := s.GetMetadata(targetCtx)
	if err != nil {
		logger.Printf("Error getting metadata for %s: %v", targetCtx, err)
		return nil, fmt.Errorf("failed to load docker context '%s': %v", targetCtx, err)
	}

	epMeta, err := docker.EndpointFromContext(meta)
	if err != nil {
		logger.Printf("EndpointFromContext failed for %s: %v", targetCtx, err)
		return nil, fmt.Errorf("failed to parse endpoint for context '%s': %v", targetCtx, err)
	}

	ep, err := docker.WithTLSData(s, targetCtx, epMeta)
	if err != nil {
		logger.Printf("TLS data loading failed (non-critical): %v", err)
		ep = docker.Endpoint{EndpointMeta: epMeta}
	}

	logger.Printf("Using Host: %s", ep.Host)
	opts := append(baseOpts, client.WithHost(ep.Host))

	if ep.TLSData != nil {
		httpClient, err := newTLSClient(ep.TLSData, ep.SkipTLSVerify)
		if err != nil {
			return nil, err
		}
		opts = append(opts, client.WithHTTPClient(httpClient))
	}

	return opts, nil
}

func newTLSClient(tlsData *clicontext.TLSData, skipVerify bool) (*http.Client, error) {
	tlsConfig := &tls.Config{
		InsecureSkipVerify: skipVerify,
	}

	if tlsData.CA != nil {
		certPool := x509.NewCertPool()
		certPool.AppendCertsFromPEM(tlsData.CA)
		tlsConfig.RootCAs = certPool
	}

	if tlsData.Cert != nil && tlsData.Key != nil {
		cert, err := tls.X509KeyPair(tlsData.Cert, tlsData.Key)
		if err != nil {
			return nil, err
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}

	transport := &http.Transport{
		TLSClientConfig: tlsConfig,
	}
	return &http.Client{Transport: transport}, nil
}

func (d *DockerClient) ListContainers() ([]common.Resource, error) {
	return d.Container.List()
}

func (d *DockerClient) ListImages() ([]common.Resource, error) {
	return d.Image.List()
}

func (d *DockerClient) ListVolumes() ([]common.Resource, error) {
	return d.Volume.List()
}

func (d *DockerClient) ListNetworks() ([]common.Resource, error) {
	return d.Network.List()
}

func (d *DockerClient) ListServices() ([]common.Resource, error) {
	return d.Service.List()
}

func (d *DockerClient) ListNodes() ([]common.Resource, error) {
	return d.Node.List()
}

func (d *DockerClient) ListCompose() ([]common.Resource, error) {
	return d.Compose.List()
}

func (d *DockerClient) ListSecrets() ([]common.Resource, error) {
	return d.Secret.List()
}

// Actions wrappers
func (d *DockerClient) StopContainer(id string) error {
	return d.Container.Stop(id)
}

func (d *DockerClient) StartContainer(id string) error {
	return d.Container.Start(id)
}

func (d *DockerClient) RestartContainer(id string) error {
	return d.Container.Restart(id)
}

func (d *DockerClient) RemoveContainer(id string, force bool) error {
	return d.Container.Remove(id, force)
}

func (d *DockerClient) PruneContainers() error {
	return d.Container.Prune()
}

func (d *DockerClient) RemoveImage(id string, force bool) error {
	return d.Image.Remove(id, force)
}

func (d *DockerClient) PruneImages() error {
	return d.Image.Prune()
}

func (d *DockerClient) PullImage(tag string) error {
	return d.Image.Pull(tag)
}

func (d *DockerClient) CreateVolume(name string) error {
	return d.Volume.Create(name)
}

func (d *DockerClient) RemoveVolume(id string, force bool) error {
	return d.Volume.Remove(id, force)
}

func (d *DockerClient) PruneVolumes() error {
	return d.Volume.Prune()
}

func (d *DockerClient) CreateNetwork(name string) error {
	return d.Network.Create(name)
}

func (d *DockerClient) RemoveNetwork(id string) error {
	return d.Network.Remove(id)
}

func (d *DockerClient) ConnectNetwork(networkID, containerID string) error {
	return d.Network.Connect(networkID, containerID)
}

func (d *DockerClient) DisconnectNetwork(networkID, containerID string) error {
	return d.Network.Disconnect(networkID, containerID)
}

func (d *DockerClient) PruneNetworks() error {
	return d.Network.Prune()
}

func (d *DockerClient) ScaleService(id string, replicas uint64) error {
	return d.Service.Scale(id, replicas)
}

func (d *DockerClient) UpdateServiceImage(id string, image string) error {
	return d.Service.UpdateImage(id, image)
}

func (d *DockerClient) RemoveService(id string) error {
	return d.Service.Remove(id)
}

func (d *DockerClient) RemoveNode(id string, force bool) error {
	return d.Node.Remove(id, force)
}

func (d *DockerClient) RemoveSecret(id string) error {
	return d.Secret.Remove(id)
}

func (d *DockerClient) CreateSecret(name string, data []byte) error {
	return d.Secret.Create(name, data)
}

func (d *DockerClient) StopComposeProject(projectName string) error {
	return d.Compose.Stop(projectName)
}

func (d *DockerClient) RestartComposeProject(projectName string) error {
	return d.Compose.Restart(projectName)
}

func (d *DockerClient) GetComposeConfig(projectName string) (string, error) {
	return d.Compose.GetConfig(projectName)
}

// Common/Stats wrappers
func (d *DockerClient) GetHostStats() (common.HostStats, error) {
	return common.GetHostStats(d.Cli, d.Ctx, d.ContextName)
}

func (d *DockerClient) GetHostStatsWithUsage() (common.HostStats, error) {
	return common.GetHostStatsWithUsage(d.Cli, d.Ctx, d.ContextName)
}

func (d *DockerClient) Inspect(resourceType, id string) (string, error) {
	return common.Inspect(d.Cli, d.Ctx, resourceType, id)
}

func (d *DockerClient) GetContainerStats(id string) (string, error) {
	return common.GetContainerStats(d.Cli, d.Ctx, id)
}

func (d *DockerClient) GetContainerEnv(id string) ([]string, error) {
	return d.Container.GetEnv(id)
}

func (d *DockerClient) HasTTY(id string) (bool, error) {
	return common.HasTTY(d.Cli, d.Ctx, id)
}

func (d *DockerClient) GetContainerLogs(id string, since string, tail string, timestamps bool) (io.ReadCloser, error) {
	return d.Container.Logs(id, since, tail, timestamps)
}

func (d *DockerClient) GetServiceLogs(id string, since string, tail string, timestamps bool) (io.ReadCloser, error) {
	return d.Service.Logs(id, since, tail, timestamps)
}

func (d *DockerClient) GetServiceEnv(id string) ([]string, error) {
	return d.Service.GetEnv(id)
}

func (d *DockerClient) GetServiceSecrets(id string) ([]*swarm.SecretReference, error) {
	return d.Service.GetSecrets(id)
}

func (d *DockerClient) SetServiceSecrets(id string, secretRefs []*swarm.SecretReference) error {
	return d.Service.SetSecrets(id, secretRefs)
}

func (d *DockerClient) GetServiceNetworks(id string) ([]swarm.NetworkAttachmentConfig, error) {
	return d.Service.GetNetworks(id)
}

func (d *DockerClient) SetServiceNetworks(id string, networks []swarm.NetworkAttachmentConfig) error {
	return d.Service.SetNetworks(id, networks)
}


func (d *DockerClient) ListServicesForSecret(secretID string) ([]common.Resource, error) {
	services, err := d.Service.List()
	if err != nil {
		return nil, err
	}

	var filtered []common.Resource
	for _, svc := range services {
		// Check if this service uses the secret
		secrets, err := d.Service.GetSecrets(svc.GetID())
		if err == nil {
			for _, s := range secrets {
				if s.SecretID == secretID {
					filtered = append(filtered, svc)
					break
				}
			}
		}
	}

	return filtered, nil
}

func (d *DockerClient) GetComposeLogs(projectName string, since string, tail string, timestamps bool) (io.ReadCloser, error) {
	return d.Compose.Logs(projectName, since, tail, timestamps)
}

func (d *DockerClient) ListTasksForNode(nodeID string) ([]swarm.Task, error) {
	return d.Node.ListTasks(nodeID)
}

func (d *DockerClient) ListTasksForService(serviceID string) ([]swarm.Task, error) {
	return d.Service.ListTasks(serviceID)
}

func (d *DockerClient) ListVolumesForContainer(id string) ([]common.Resource, error) {
	// Inspect container to get mounts
	json, err := d.Cli.ContainerInspect(d.Ctx, id)
	if err != nil {
		return nil, err
	}
	
	names := make(map[string]bool)
	for _, m := range json.Mounts {
		if m.Type == "volume" {
			names[m.Name] = true
		}
	}
	
	// List all volumes and filter
	all, err := d.Volume.List()
	if err != nil {
		return nil, err
	}
	
	var filtered []common.Resource
	for _, r := range all {
		if v, ok := r.(volume.Volume); ok {
			if names[v.Name] {
				// Get destination from map (if I update map to store it)
				dest := ""
				for _, m := range json.Mounts {
					if m.Type == "volume" && m.Name == v.Name {
						dest = m.Destination
						break
					}
				}

				cv := volume.ContainerVolume{
					Volume:      v,
					Destination: dest,
				}
				filtered = append(filtered, cv)
			}
		}
	}
	
	return filtered, nil
}

func (d *DockerClient) ListNetworksForContainer(id string) ([]common.Resource, error) {
	// Inspect container to get networks
	json, err := d.Cli.ContainerInspect(d.Ctx, id)
	if err != nil {
		return nil, err
	}
	
	ids := make(map[string]bool)
	for _, n := range json.NetworkSettings.Networks {
		ids[n.NetworkID] = true
	}
	
	// List all networks and filter
	all, err := d.Network.List()
	if err != nil {
		return nil, err
	}
	
	var filtered []common.Resource
	for _, r := range all {
		if n, ok := r.(network.Network); ok {
			if ids[n.ID] {
				filtered = append(filtered, r)
			}
		}
	}
	
	return filtered, nil
}
