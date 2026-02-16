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
	"strings"
	"sync"
	"time"

	"github.com/docker/cli/cli/config"
	clicontext "github.com/docker/cli/cli/context"
	"github.com/docker/cli/cli/context/docker"
	"github.com/docker/cli/cli/context/store"
	dcontainer "github.com/docker/docker/api/types/container"
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

// Cached container info for instant scoped queries (drill-down)
type containerInfoCache struct {
	Mounts []mountInfoCache
	NetIDs map[string]bool
}

type mountInfoCache struct {
	Type        string
	Name        string
	Source      string
	Destination string
}

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

	// Resource cache for fast scoped queries
	cacheMu          sync.RWMutex
	volumeCache      []common.Resource            // Raw Volume.List() results
	networkCache     []common.Resource            // Network.List() results
	containerInfoMap map[string]containerInfoCache // containerID -> mount/network info
}

func NewDockerClient(contextName string, apiTimeout time.Duration) (*DockerClient, error) {
	logger, cleanup := initLogger()
	defer cleanup()

	ctxName, opts, err := resolveClientOpts(contextName, logger, apiTimeout)
	if err != nil {
		return nil, err
	}

	cli, err := client.NewClientWithOpts(opts...)
	if err != nil {
		return nil, err
	}
	ctx := context.Background()
	
	return &DockerClient{
		Cli:              cli,
		Ctx:              ctx,
		ContextName:      ctxName,
		Container:        container.NewManager(cli, ctx),
		Image:            image.NewManager(cli, ctx),
		Volume:           volume.NewManager(cli, ctx),
		Network:          network.NewManager(cli, ctx),
		Service:          service.NewManager(cli, ctx),
		Node:             node.NewManager(cli, ctx),
		Secret:           secret.NewManager(cli, ctx),
		Compose:          compose.NewManager(cli, ctx),
		containerInfoMap: make(map[string]containerInfoCache),
	}, nil
}

func initLogger() (*log.Logger, func()) {
	f, err := os.OpenFile("/tmp/d4s_debug_dao.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return log.New(io.Discard, "", 0), func() {}
	}
	return log.New(f, "d4s-dao: ", log.LstdFlags), func() { f.Close() }
}

func resolveClientOpts(flagContext string, logger *log.Logger, apiTimeout time.Duration) (string, []client.Opt, error) {
	opts := []client.Opt{
		client.WithAPIVersionNegotiation(),
	}

	// 1. Flag takes precedence
	if flagContext != "" {
		logger.Printf("Explicit context requested via flag: %s", flagContext)
		opts, err := loadSpecificContext(flagContext, logger, opts, apiTimeout)
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
	opts, err := loadSpecificContext(targetCtx, logger, opts, apiTimeout)
	return targetCtx, opts, err
}

func loadSpecificContext(targetCtx string, logger *log.Logger, baseOpts []client.Opt, apiTimeout time.Duration) ([]client.Opt, error) {
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
		// Only TLS connections get a custom HTTP client (they use TCP, not Unix sockets).
		// Calling WithHTTPClient on a Unix socket context would destroy the
		// socket transport that the Docker SDK configures internally.
		httpClient, err := newTLSClient(ep.TLSData, ep.SkipTLSVerify, apiTimeout)
		if err != nil {
			return nil, err
		}
		opts = append(opts, client.WithHTTPClient(httpClient))
	}

	return opts, nil
}

func newTLSClient(tlsData *clicontext.TLSData, skipVerify bool, timeout time.Duration) (*http.Client, error) {
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
	return &http.Client{Transport: transport, Timeout: timeout}, nil
}

func (d *DockerClient) ListContainers() ([]common.Resource, error) {
	return d.Container.List()
}

func (d *DockerClient) ListImages() ([]common.Resource, error) {
	return d.Image.List()
}

func (d *DockerClient) ListVolumes() ([]common.Resource, error) {
	vols, err := d.Volume.List()
	if err != nil {
		return nil, err
	}

	// Cache raw volume list for scoped queries (avoids DiskUsage on drill-down)
	d.cacheMu.Lock()
	d.volumeCache = vols
	d.cacheMu.Unlock()

	// Build volume name -> container names mapping
	usageMap := make(map[string][]string)
	containers, err := d.Cli.ContainerList(d.Ctx, dcontainer.ListOptions{All: true})
	if err == nil {
		// Cache container mount/network info for instant drill-down
		infoMap := make(map[string]containerInfoCache, len(containers))

		for _, c := range containers {
			name := ""
			if len(c.Names) > 0 {
				name = strings.TrimPrefix(c.Names[0], "/")
			}
			if name == "" {
				name = c.ID[:12]
			}

			info := containerInfoCache{NetIDs: make(map[string]bool)}

			for _, m := range c.Mounts {
				if m.Type == "volume" {
					usageMap[m.Name] = append(usageMap[m.Name], name)
				}
				info.Mounts = append(info.Mounts, mountInfoCache{
					Type:        string(m.Type),
					Name:        m.Name,
					Source:      m.Source,
					Destination: m.Destination,
				})
			}
			if c.NetworkSettings != nil {
				for _, n := range c.NetworkSettings.Networks {
					info.NetIDs[n.NetworkID] = true
				}
			}

			infoMap[c.ID] = info
		}

		d.cacheMu.Lock()
		d.containerInfoMap = infoMap
		d.cacheMu.Unlock()
	}

	// Enrich volumes with usage info
	for i, r := range vols {
		if v, ok := r.(volume.Volume); ok {
			if names, found := usageMap[v.Name]; found {
				v.UsedBy = strings.Join(names, ", ")
			}
			vols[i] = v
		}
	}

	return vols, nil
}

func (d *DockerClient) ListNetworks() ([]common.Resource, error) {
	result, err := d.Network.List()
	if err != nil {
		return nil, err
	}

	d.cacheMu.Lock()
	d.networkCache = result
	d.cacheMu.Unlock()

	return result, nil
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

func (d *DockerClient) RestartService(id string) error {
	return d.Service.Restart(id)
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

func (d *DockerClient) SetServiceEnv(id string, env []string) error {
	return d.Service.SetEnv(id, env)
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
	// 1. Get container mount info (cache first, API fallback)
	d.cacheMu.RLock()
	info, cached := d.containerInfoMap[id]
	d.cacheMu.RUnlock()

	if !cached {
		cj, err := d.Cli.ContainerInspect(d.Ctx, id)
		if err != nil {
			return nil, err
		}
		info = containerInfoCache{NetIDs: make(map[string]bool)}
		for _, m := range cj.Mounts {
			info.Mounts = append(info.Mounts, mountInfoCache{
				Type:        string(m.Type),
				Name:        m.Name,
				Source:      m.Source,
				Destination: m.Destination,
			})
		}
		if cj.NetworkSettings != nil {
			for _, n := range cj.NetworkSettings.Networks {
				info.NetIDs[n.NetworkID] = true
			}
		}
		d.cacheMu.Lock()
		d.containerInfoMap[id] = info
		d.cacheMu.Unlock()
	}

	// 2. Get volume list (cache first, API fallback)
	d.cacheMu.RLock()
	cachedVols := d.volumeCache
	d.cacheMu.RUnlock()

	if cachedVols == nil {
		cachedVols, _ = d.Volume.List()
	}

	volumeIndex := make(map[string]volume.Volume)
	for _, r := range cachedVols {
		if v, ok := r.(volume.Volume); ok {
			volumeIndex[v.Name] = v
		}
	}

	// 3. Match mounts to volumes
	var result []common.Resource
	for _, m := range info.Mounts {
		mountType := strings.ToUpper(m.Type)

		switch m.Type {
		case "volume":
			if v, ok := volumeIndex[m.Name]; ok {
				result = append(result, volume.ContainerVolume{
					Volume:      v,
					Destination: m.Destination,
					Type:        mountType,
				})
			}
		default:
			result = append(result, volume.ContainerVolume{
				Volume: volume.Volume{
					Name:    m.Source,
					Driver:  "-",
					Scope:   "-",
					Mount:   m.Source,
					Created: "-",
					Size:    "-",
				},
				Destination: m.Destination,
				Type:        mountType,
			})
		}
	}

	return result, nil
}

func (d *DockerClient) ListNetworksForContainer(id string) ([]common.Resource, error) {
	// 1. Get container network info (cache first, API fallback)
	d.cacheMu.RLock()
	info, cached := d.containerInfoMap[id]
	d.cacheMu.RUnlock()

	if !cached {
		cj, err := d.Cli.ContainerInspect(d.Ctx, id)
		if err != nil {
			return nil, err
		}
		info = containerInfoCache{NetIDs: make(map[string]bool)}
		if cj.NetworkSettings != nil {
			for _, n := range cj.NetworkSettings.Networks {
				info.NetIDs[n.NetworkID] = true
			}
		}
		for _, m := range cj.Mounts {
			info.Mounts = append(info.Mounts, mountInfoCache{
				Type:        string(m.Type),
				Name:        m.Name,
				Source:      m.Source,
				Destination: m.Destination,
			})
		}
		d.cacheMu.Lock()
		d.containerInfoMap[id] = info
		d.cacheMu.Unlock()
	}

	// 2. Get network list (cache first, API fallback)
	d.cacheMu.RLock()
	cachedNets := d.networkCache
	d.cacheMu.RUnlock()

	if cachedNets == nil {
		var err error
		cachedNets, err = d.Network.List()
		if err != nil {
			return nil, err
		}
	}

	// 3. Filter networks by container membership
	var filtered []common.Resource
	for _, r := range cachedNets {
		if n, ok := r.(network.Network); ok {
			if info.NetIDs[n.ID] {
				filtered = append(filtered, r)
			}
		}
	}

	return filtered, nil
}
