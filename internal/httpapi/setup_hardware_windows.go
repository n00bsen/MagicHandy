//go:build windows

package httpapi

import (
	"encoding/binary"
	"strings"

	"golang.org/x/sys/windows/registry"
)

// displayAdapterClassKey is the Windows device-class key for display adapters.
const displayAdapterClassKey = `SYSTEM\CurrentControlSet\Control\Class\{4d36e968-e325-11ce-bfc1-08002be10318}`

// displayAdapters reads adapter names and dedicated memory from the driver
// registry entries. Win32_VideoController.AdapterRAM is a 32-bit field that
// caps at 4 GiB, so the 64-bit HardwareInformation.qwMemorySize is preferred.
func displayAdapters() []displayAdapter {
	class, err := registry.OpenKey(registry.LOCAL_MACHINE, displayAdapterClassKey, registry.ENUMERATE_SUB_KEYS)
	if err != nil {
		return nil
	}
	defer class.Close()
	names, err := class.ReadSubKeyNames(0)
	if err != nil {
		return nil
	}
	var adapters []displayAdapter
	for _, name := range names {
		// Adapter instances are numbered 0000, 0001, ...; "Properties" and
		// "Configuration" are not adapters and are usually access-denied.
		if len(name) != 4 {
			continue
		}
		key, err := registry.OpenKey(class, name, registry.QUERY_VALUE)
		if err != nil {
			continue
		}
		adapter := displayAdapter{}
		adapter.Name, _, _ = key.GetStringValue("DriverDesc")
		adapter.Vendor, _, _ = key.GetStringValue("ProviderName")
		adapter.MemoryBytes = adapterMemoryBytes(key)
		_ = key.Close()
		if strings.TrimSpace(adapter.Name) != "" {
			adapters = append(adapters, adapter)
		}
	}
	return adapters
}

func adapterMemoryBytes(key registry.Key) uint64 {
	if memory, _, err := key.GetIntegerValue("HardwareInformation.qwMemorySize"); err == nil {
		return memory
	}
	if raw, _, err := key.GetBinaryValue("HardwareInformation.qwMemorySize"); err == nil && len(raw) >= 8 {
		return binary.LittleEndian.Uint64(raw)
	}
	if memory, _, err := key.GetIntegerValue("HardwareInformation.MemorySize"); err == nil {
		return memory
	}
	// Some drivers store the 32-bit size as REG_BINARY.
	if raw, _, err := key.GetBinaryValue("HardwareInformation.MemorySize"); err == nil && len(raw) >= 4 {
		return uint64(binary.LittleEndian.Uint32(raw))
	}
	return 0
}
