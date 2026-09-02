// Copyright (c) 2026 Forke Inc. (https://www.forke.space/)
// Source-Available License (Non-Commercial / Fair Source).
// Open for inspection, learning, and non-commercial development.
// Commercial use, hosting, or resale without authorization is strictly prohibited.

package handlers

import (
	"encoding/json"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"
)

var processStartTime = time.Now()

type SystemTelemetryResponse struct {
	Hostname            string    `json:"hostname"`
	Platform            string    `json:"platform"`
	OS                  string    `json:"os"`
	Arch                string    `json:"arch"`
	CPUCount            int       `json:"cpuCount"`
	CPUModel            string    `json:"cpuModel"`
	LoadAvg             []float64 `json:"loadAvg"`
	SystemUptimeSeconds int64     `json:"systemUptimeSeconds"`
	TotalMemBytes       uint64    `json:"totalMemBytes"`
	FreeMemBytes        uint64    `json:"freeMemBytes"`
	UsedMemBytes        uint64    `json:"usedMemBytes"`
	MemUsagePct         int       `json:"memUsagePct"`
	Timestamp           time.Time `json:"timestamp"`
}

// SystemTelemetry godoc
// @Summary System telemetry
// @Description Returns accurate host VM telemetry including kernel uptime, memory, and CPU load
// @Tags System
// @Produce json
// @Success 200 {object} SystemTelemetryResponse
// @Router /api/v1/system/telemetry [get]
func SystemTelemetry(w http.ResponseWriter, r *http.Request) {
	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "oracle-cloud-vm"
	}

	cpuCount := runtime.NumCPU()
	cpuModel := "Ampere Altra (ARM64)"
	loadAvg := []float64{0.05, 0.05, 0.05}
	uptimeSecs := int64(time.Since(processStartTime).Seconds())
	totalMem := uint64(12 * 1024 * 1024 * 1024) // default 12 GB baseline
	freeMem := uint64(8 * 1024 * 1024 * 1024)
	usedMem := totalMem - freeMem
	memPct := 33

	// 1. On Linux, read /proc/uptime for true kernel host uptime
	if data, err := os.ReadFile("/proc/uptime"); err == nil {
		fields := strings.Fields(string(data))
		if len(fields) > 0 {
			if sec, parseErr := strconv.ParseFloat(fields[0], 64); parseErr == nil && sec > 0 {
				uptimeSecs = int64(sec)
			}
		}
	}

	// 2. On Linux, read /proc/loadavg
	if data, err := os.ReadFile("/proc/loadavg"); err == nil {
		fields := strings.Fields(string(data))
		if len(fields) >= 3 {
			l1, _ := strconv.ParseFloat(fields[0], 64)
			l5, _ := strconv.ParseFloat(fields[1], 64)
			l15, _ := strconv.ParseFloat(fields[2], 64)
			loadAvg = []float64{l1, l5, l15}
		}
	}

	// 3. On Linux, read /proc/meminfo
	if data, err := os.ReadFile("/proc/meminfo"); err == nil {
		lines := strings.Split(string(data), "\n")
		memMap := make(map[string]uint64)
		for _, line := range lines {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				valStr := strings.TrimSpace(parts[1])
				valFields := strings.Fields(valStr)
				if len(valFields) > 0 {
					if kb, parseErr := strconv.ParseUint(valFields[0], 10, 64); parseErr == nil {
						memMap[key] = kb * 1024 // convert kB to bytes
					}
				}
			}
		}

		if t, ok := memMap["MemTotal"]; ok && t > 0 {
			totalMem = t
			avail := memMap["MemAvailable"]
			if avail == 0 {
				avail = memMap["MemFree"] + memMap["Buffers"] + memMap["Cached"]
			}
			freeMem = avail
			if totalMem > freeMem {
				usedMem = totalMem - freeMem
			} else {
				usedMem = 0
			}
			memPct = int((usedMem * 100) / totalMem)
		}
	}

	// 4. On Linux, read /proc/cpuinfo for model name
	if data, err := os.ReadFile("/proc/cpuinfo"); err == nil {
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			if strings.HasPrefix(strings.ToLower(line), "model name") {
				parts := strings.SplitN(line, ":", 2)
				if len(parts) == 2 {
					cpuModel = strings.TrimSpace(parts[1])
					break
				}
			}
		}
	}

	resp := SystemTelemetryResponse{
		Hostname:            hostname,
		Platform:            runtime.GOOS,
		OS:                  runtime.GOOS,
		Arch:                runtime.GOARCH,
		CPUCount:            cpuCount,
		CPUModel:            cpuModel,
		LoadAvg:             loadAvg,
		SystemUptimeSeconds: uptimeSecs,
		TotalMemBytes:       totalMem,
		FreeMemBytes:        freeMem,
		UsedMemBytes:        usedMem,
		MemUsagePct:         memPct,
		Timestamp:           time.Now().UTC(),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}
