package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/load"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/net"
)

//go:embed static/*
var staticFiles embed.FS

type Stats struct {
	Hostname   string        `json:"hostname"`
	CPUPercent float64       `json:"cpu_percent"`
	Memory     MemoryStats   `json:"memory"`
	Disk       DiskStats     `json:"disk"`
	Network    NetworkStats  `json:"network"`
	Load       LoadStats     `json:"load"`
	Uptime     string        `json:"uptime"`
	Timestamp  time.Time     `json:"timestamp"`
}

type MemoryStats struct {
	Total   uint64  `json:"total"`
	Used    uint64  `json:"used"`
	Percent float64 `json:"percent"`
}

type DiskStats struct {
	Total   uint64  `json:"total"`
	Used    uint64  `json:"used"`
	Percent float64 `json:"percent"`
}

type NetworkStats struct {
	BytesSent uint64 `json:"bytes_sent"`
	BytesRecv uint64 `json:"bytes_recv"`
}

type LoadStats struct {
	Load1  float64 `json:"1min"`
	Load5  float64 `json:"5min"`
	Load15 float64 `json:"15min"`
}

func formatUptime(seconds uint64) string {
	days := seconds / 86400
	hours := (seconds % 86400) / 3600
	minutes := (seconds % 3600) / 60

	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm", days, hours, minutes)
	} else if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, minutes)
	}
	return fmt.Sprintf("%dm", minutes)
}

func getStats() (*Stats, error) {
	// Hostname
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}

	// CPU
	cpuPercent, err := cpu.Percent(time.Second, false)
	if err != nil {
		return nil, fmt.Errorf("cpu: %w", err)
	}
	cpuPct := 0.0
	if len(cpuPercent) > 0 {
		cpuPct = cpuPercent[0]
	}

	// Memory
	memInfo, err := mem.VirtualMemory()
	if err != nil {
		return nil, fmt.Errorf("memory: %w", err)
	}

	// Disk
	diskInfo, err := disk.Usage("/")
	if err != nil {
		return nil, fmt.Errorf("disk: %w", err)
	}

	// Network
	netInfo, err := net.IOCounters(false)
	if err != nil {
		return nil, fmt.Errorf("network: %w", err)
	}
	var bytesSent, bytesRecv uint64
	if len(netInfo) > 0 {
		bytesSent = netInfo[0].BytesSent
		bytesRecv = netInfo[0].BytesRecv
	}

	// Load
	loadInfo, err := load.Avg()
	if err != nil {
		return nil, fmt.Errorf("load: %w", err)
	}

	// Uptime
	hostInfo, err := host.Info()
	if err != nil {
		return nil, fmt.Errorf("host: %w", err)
	}

	stats := &Stats{
		Hostname:   hostname,
		CPUPercent: float64(int(cpuPct*10)) / 10, // Round to 1 decimal
		Memory: MemoryStats{
			Total:   memInfo.Total,
			Used:    memInfo.Used,
			Percent: float64(int(memInfo.UsedPercent*10)) / 10,
		},
		Disk: DiskStats{
			Total:   diskInfo.Total,
			Used:    diskInfo.Used,
			Percent: float64(int(diskInfo.UsedPercent*10)) / 10,
		},
		Network: NetworkStats{
			BytesSent: bytesSent,
			BytesRecv: bytesRecv,
		},
		Load: LoadStats{
			Load1:  float64(int(loadInfo.Load1*100)) / 100,
			Load5:  float64(int(loadInfo.Load5*100)) / 100,
			Load15: float64(int(loadInfo.Load15*100)) / 100,
		},
		Uptime:    formatUptime(hostInfo.Uptime),
		Timestamp: time.Now(),
	}

	return stats, nil
}

func statsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	stats, err := getStats()
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error": "%s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(stats)
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}

	// Serve static files
	http.Handle("/", http.FileServer(http.FS(staticFiles)))

	// API endpoint
	http.HandleFunc("/api/stats", statsHandler)

	log.Printf("Server dashboard running on http://0.0.0.0:%s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
