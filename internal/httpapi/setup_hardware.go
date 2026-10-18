package httpapi

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

func setupHardwareSnapshot() map[string]any {
	snapshot := map[string]any{
		"platform": runtime.GOOS + "/" + runtime.GOARCH,
		"nvidia":   false,
		"cuda":     false,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, "nvidia-smi", "--query-gpu=name,memory.total", "--format=csv,noheader,nounits") // #nosec G204 -- fixed diagnostic command.
	configureSetupProcess(command)
	if output, err := command.Output(); err == nil {
		name, memory := largestNVIDIAGPU(string(output))
		if name != "" {
			snapshot["nvidia"] = true
			snapshot["gpu_name"] = name
		}
		if memory != "" {
			snapshot["vram_mib"] = memory
		}
	}
	if _, err := exec.LookPath("nvcc"); err == nil {
		snapshot["cuda"] = true
	}
	// Managed llama.cpp ships CPU and CUDA backends only, so an AMD card runs
	// local chat through Ollama. Setup reports it so it can offer that path.
	if nvidia, _ := snapshot["nvidia"].(bool); !nvidia {
		if name, memory := largestAMDGPU(displayAdapters()); name != "" {
			snapshot["amd"] = true
			snapshot["gpu_name"] = name
			if memory != "" {
				snapshot["vram_mib"] = memory
			}
		}
	}
	snapshot["ollama"] = ollamaInstalled()
	return snapshot
}

// displayAdapter is one display adapter as the OS reports it.
type displayAdapter struct {
	Name        string
	Vendor      string
	MemoryBytes uint64
}

// largestAMDGPU picks the AMD adapter with the most dedicated memory, so a
// machine with an integrated Radeon and a discrete card is judged by the
// discrete one. Memory is "" when the driver does not report it.
func largestAMDGPU(adapters []displayAdapter) (string, string) {
	bestName, bestMemory, found := "", uint64(0), false
	for _, adapter := range adapters {
		name := strings.TrimSpace(adapter.Name)
		if name == "" || !isAMDAdapter(name, adapter.Vendor) {
			continue
		}
		if !found || adapter.MemoryBytes > bestMemory {
			bestName, bestMemory, found = name, adapter.MemoryBytes, true
		}
	}
	if mib := bestMemory / (1024 * 1024); found && mib > 0 {
		return bestName, strconv.FormatUint(mib, 10)
	}
	return bestName, ""
}

func isAMDAdapter(name, vendor string) bool {
	vendor = strings.ToLower(strings.TrimSpace(vendor))
	return strings.Contains(vendor, "advanced micro devices") || vendor == "amd" || strings.HasPrefix(vendor, "amd ") ||
		strings.Contains(strings.ToLower(name), "radeon")
}

// ollamaInstalled reports whether an Ollama executable is on PATH or in the
// per-user location its Windows installer uses.
func ollamaInstalled() bool {
	if _, err := exec.LookPath("ollama"); err == nil {
		return true
	}
	if runtime.GOOS != "windows" {
		return false
	}
	base := os.Getenv("LOCALAPPDATA")
	if base == "" {
		return false
	}
	info, err := os.Stat(filepath.Join(base, "Programs", "Ollama", "ollama.exe"))
	return err == nil && !info.IsDir()
}

// largestNVIDIAGPU picks the card with the most memory from nvidia-smi's
// "name, MiB" lines, so a machine with two GPUs is judged by the one a model
// would load on rather than by an unparsable joined line.
func largestNVIDIAGPU(output string) (string, string) {
	bestName, bestMemory, bestMiB := "", "", -1
	for _, line := range strings.Split(strings.ReplaceAll(output, "\r", ""), "\n") {
		name, memory, found := strings.Cut(line, ",")
		name, memory = strings.TrimSpace(name), strings.TrimSpace(memory)
		if name == "" {
			continue
		}
		mib := 0
		if found {
			if value, err := strconv.Atoi(memory); err == nil && value > 0 {
				mib = value
			} else {
				memory = ""
			}
		}
		if mib > bestMiB {
			bestName, bestMemory, bestMiB = name, memory, mib
		}
	}
	return bestName, bestMemory
}
