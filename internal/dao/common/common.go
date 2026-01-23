package common

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/client"
	"github.com/gdamore/tcell/v2"
	"github.com/jr-k/d4s/internal/buildinfo"
	"golang.org/x/net/context"
)

// Re-export common types
type Resource interface {
	GetID() string
	GetCells() []string
	GetStatusColor() (statusColor tcell.Color, hoverFgColor tcell.Color)
	// GetColumnValue returns the raw value for a specific column name
	GetColumnValue(columnName string) string
	// GetDefaultColumn returns the column name to use if no specific column is selected or relevant (for Copy)
	GetDefaultColumn() string
	// GetDefaultSortColumn returns the column name to use for default sorting
	GetDefaultSortColumn() string
}

// HostStats represents basic host metrics
type HostStats struct {
	CPU        string
	CPUPercent string
	Mem        string
	MemPercent string
	Name       string
	Version    string
	Context    string
	User       string
	Hostname   string
	D4SVersion string
	LatestVersion string
}

func GetHostStats(cli *client.Client, ctx context.Context, contextName string) (HostStats, error) {
	info, err := cli.Info(ctx)
	if err != nil {
		return HostStats{}, err
	}
	
	memTotal := FormatBytes(info.MemTotal)
	
	// Get current user
	user := os.Getenv("USER")
	if user == "" {
		user = os.Getenv("USERNAME") // Windows fallback
	}
	if user == "" {
		user = "unknown"
	}
	
	// Get hostname
	hostname, _ := os.Hostname()

	return HostStats{
		CPU:        fmt.Sprintf("%d", info.NCPU),
		CPUPercent: "...", // Placeholder
		Mem:        memTotal,
		MemPercent: "...", // Placeholder
		Name:       info.Name,
		Version:    info.ServerVersion,
		Context:    contextName,
		User:       user,
		Hostname:   hostname,
		D4SVersion: buildinfo.Version,
	}, nil
}

func GetHostStatsWithUsage(cli *client.Client, ctx context.Context, contextName string) (HostStats, error) {
	// First get basic stats
	stats, err := GetHostStats(cli, ctx, contextName)
	if err != nil {
		return stats, err
	}
	
	// Then calculate usage stats asynchronously
	info, _ := cli.Info(ctx)
	containers, err := cli.ContainerList(ctx, container.ListOptions{All: false})
	if err != nil || len(containers) == 0 {
		return stats, nil
	}
	
	var totalCPU float64
	var totalMem uint64
	validStats := 0
	
	// Collect stats from first few containers (to avoid blocking too long)
	maxContainers := len(containers)
	if maxContainers > 10 {
		maxContainers = 10 // Limit to 10 containers for performance
	}
	
	for i := 0; i < maxContainers; i++ {
		c := containers[i]
		statsResp, err := cli.ContainerStats(ctx, c.ID, false)
		if err != nil {
			continue
		}
		
		cpuPct, mem, _ := CalculateContainerStats(statsResp.Body)
		totalCPU += cpuPct
		if mem > 0 {
			totalMem += mem
			validStats++
		}
	}
	
	// Format CPU percentage
	if validStats > 0 && totalCPU > 0 {
		stats.CPUPercent = fmt.Sprintf("%.1f%%", totalCPU)
	} else {
		stats.CPUPercent = "0%"
	}
	
	// Format Memory percentage
	if info.MemTotal > 0 && totalMem > 0 {
		memPct := float64(totalMem) / float64(info.MemTotal) * 100.0
		stats.MemPercent = fmt.Sprintf("%.1f%%", memPct)
	} else {
		stats.MemPercent = "0%"
	}
	
	return stats, nil
}

func Inspect(cli *client.Client, ctx context.Context, resourceType, id string) (string, error) {
	var data interface{}
	var err error

	switch resourceType {
	case "container":
		data, err = cli.ContainerInspect(ctx, id)
	case "image":
		data, _, err = cli.ImageInspectWithRaw(ctx, id)
	case "volume":
		data, err = cli.VolumeInspect(ctx, id)
	case "network":
		data, err = cli.NetworkInspect(ctx, id, network.InspectOptions{})
	case "service":
		data, _, err = cli.ServiceInspectWithRaw(ctx, id, swarm.ServiceInspectOptions{})
	case "node":
		data, _, err = cli.NodeInspectWithRaw(ctx, id)
	default:
		return "", fmt.Errorf("unknown resource type: %s", resourceType)
	}

	if err != nil {
		return "", err
	}

	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func GetContainerStats(cli *client.Client, ctx context.Context, id string) (string, error) {
	resp, err := cli.ContainerStats(ctx, id, false)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var v interface{}
	if err := json.NewDecoder(resp.Body).Decode(&v); err != nil {
		return "", err
	}
	
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func HasTTY(cli *client.Client, ctx context.Context, id string) (bool, error) {
	c, err := cli.ContainerInspect(ctx, id)
	if err != nil {
		return false, err
	}
	return c.Config.Tty, nil
}

