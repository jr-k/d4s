package common

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

func ShortenPath(path string) string {
	home, err := os.UserHomeDir()
	if err == nil && home != "" && strings.HasPrefix(path, home) {
		return "~" + strings.TrimPrefix(path, home)
	}
	return path
}

func ParseStatus(s string) (status, age string) {
	s = strings.TrimSpace(s)
	
	if strings.HasPrefix(s, "Up") {
		status = "Up"
		rest := strings.TrimPrefix(s, "Up ")
		age = ShortenDuration(rest)
	} else if strings.HasPrefix(s, "Exited") {
		// "Exited (0) 5 minutes ago"
		parts := strings.SplitN(s, ") ", 2)
		if len(parts) == 2 {
			status = parts[0] + ")"
			age = ShortenDuration(strings.TrimSuffix(parts[1], " ago"))
		} else {
			status = "Exited"
			age = s
		}
	} else if strings.HasPrefix(s, "Created") {
		status = "Created"
		age = "-"
	} else if strings.HasPrefix(s, "Paused") {
		// "Up 2 hours (Paused)"
		if strings.Contains(s, "(Paused)") {
			status = "Paused"
			rest := strings.TrimPrefix(s, "Up ")
			rest = strings.TrimSuffix(rest, " (Paused)")
			age = ShortenDuration(rest)
			return
		}
		status = "Paused"
		age = "-"
	} else if strings.HasPrefix(s, "Exiting") { // Explicitly handle Exiting
		status = "Exiting"
		age = "-"
	} else if strings.Contains(strings.ToLower(s), "starting") {
		status = "Starting"
		age = "-"
	} else {
		status = s
		age = "-"
	}
	return
}

func ShortenDuration(d string) string {
	d = strings.ToLower(d)
	if strings.Contains(d, "less than") {
		return "1s"
	}
	
	// Clean up verbose words
	d = strings.ReplaceAll(d, "about ", "")
	d = strings.ReplaceAll(d, "an ", "1 ")
	d = strings.ReplaceAll(d, "a ", "1 ")
	d = strings.TrimSuffix(d, " ago")
	
	parts := strings.Fields(d)
	if len(parts) >= 2 {
		val := parts[0]
		unit := parts[1]
		
		if val == "0" && strings.HasPrefix(unit, "second") {
			return "1s"
		}

		if strings.HasPrefix(unit, "second") { return val + "s" }
		if strings.HasPrefix(unit, "minute") { return val + "m" }
		if strings.HasPrefix(unit, "hour") { return val + "h" }
		if strings.HasPrefix(unit, "day") { return val + "d" }
		if strings.HasPrefix(unit, "week") { return val + "w" }
		if strings.HasPrefix(unit, "month") { return val + "mo" }
		if strings.HasPrefix(unit, "year") { return val + "y" }
	}
	return d
}

func FormatTime(ts int64) string {
	t := time.Unix(ts, 0)
	now := time.Now()

	// Reset clock to calculate difference in calendar days
	tDate := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.Local)
	nDate := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)
	days := int(nDate.Sub(tDate).Hours() / 24)

	dateStr := t.Format("2006-01-02")
	timeStr := t.Format("15:04")

	if days == 0 {
		return fmt.Sprintf("%s %s", dateStr, timeStr)
	}
	if days == 1 {
		return fmt.Sprintf("%s %s", dateStr, timeStr)
	}

	return fmt.Sprintf("%s %s", dateStr, timeStr)
}

func FormatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(b)/float64(div), "KMGTPE"[exp])
}

func FormatBytesFixed(b int64) string {
	val := float64(b)
	// User requested: B KB MB GB TB
	units := []string{"B ", "KB", "MB", "GB", "TB", "PB", "EB"} 
	exp := 0
	for val >= 1000 && exp < len(units)-1 {
		val /= 1024
		exp++
	}
	// %3.0f: 3 digits max (since < 1000)
	// %s: unit
	return fmt.Sprintf("%3.0f %s", val, units[exp])
}

func CalculateContainerStats(body io.ReadCloser) (float64, uint64, uint64) {
	defer body.Close()
	var v map[string]interface{}
	if err := json.NewDecoder(body).Decode(&v); err != nil {
		return 0, 0, 0
	}
	cpu, mem, limit, _, _, _, _ := CalculateStatsFromMap(v)
	return cpu, mem, limit
}

func CalculateStatsFromMap(v map[string]interface{}) (float64, uint64, uint64, float64, float64, float64, float64) {
	var cpuPercent float64
	var memUsage, memLimit uint64
	var netRx, netTx, diskRead, diskWrite float64

	// CPU
	if cpuStats, ok := v["cpu_stats"].(map[string]interface{}); ok {
		// Try precpu_stats first, but it might be empty on first call with stream=false
		preCPUStats, _ := v["precpu_stats"].(map[string]interface{})
		
		if cpuUsage, ok := cpuStats["cpu_usage"].(map[string]interface{}); ok {
			var totalUsage, preTotalUsage, systemUsage, preSystemUsage float64
			
			if t, ok := cpuUsage["total_usage"].(float64); ok { totalUsage = t }
			if t, ok := cpuStats["system_cpu_usage"].(float64); ok { systemUsage = t }
			
			if preCPUStats != nil {
				if preCPUUsage, ok := preCPUStats["cpu_usage"].(map[string]interface{}); ok {
					if t, ok := preCPUUsage["total_usage"].(float64); ok { preTotalUsage = t }
				}
				if t, ok := preCPUStats["system_cpu_usage"].(float64); ok { preSystemUsage = t }
			}

			if systemUsage > 0.0 && totalUsage > 0.0 {
				cpuDelta := totalUsage - preTotalUsage
				systemDelta := systemUsage - preSystemUsage
				
				if systemDelta > 0.0 && cpuDelta > 0.0 {
					if percpu, ok := cpuUsage["percpu_usage"].([]interface{}); ok && len(percpu) > 0 {
						cpuPercent = (cpuDelta / systemDelta) * float64(len(percpu)) * 100.0
					} else if onlineCpus, ok := cpuStats["online_cpus"].(float64); ok {
						cpuPercent = (cpuDelta / systemDelta) * onlineCpus * 100.0
					} else {
						cpuPercent = (cpuDelta / systemDelta) * 100.0
					}
				}
			}
		}
	}

	// Mem
	if memStats, ok := v["memory_stats"].(map[string]interface{}); ok {
		if usage, ok := memStats["usage"].(float64); ok {
			memUsage = uint64(usage)
		}
		if limit, ok := memStats["limit"].(float64); ok {
			memLimit = uint64(limit)
		}
	}
	
	// Networks
	if networks, ok := v["networks"].(map[string]interface{}); ok {
		for _, netRaw := range networks {
			if net, ok := netRaw.(map[string]interface{}); ok {
				if rx, ok := net["rx_bytes"].(float64); ok { netRx += rx }
				if tx, ok := net["tx_bytes"].(float64); ok { netTx += tx }
			}
		}
	}
	
	// Disk (Block IO)
	if blkio, ok := v["blkio_stats"].(map[string]interface{}); ok {
		if serviceBytes, ok := blkio["io_service_bytes_recursive"].([]interface{}); ok {
			for _, itemRaw := range serviceBytes {
				if item, ok := itemRaw.(map[string]interface{}); ok {
					op := ""
					if o, ok := item["op"].(string); ok { op = strings.ToLower(o) }
					val := 0.0
					if v, ok := item["value"].(float64); ok { val = v }
					
					if op == "read" { diskRead += val }
					if op == "write" { diskWrite += val }
				}
			}
		}
	}

	return cpuPercent, memUsage, memLimit, netRx, netTx, diskRead, diskWrite
}
