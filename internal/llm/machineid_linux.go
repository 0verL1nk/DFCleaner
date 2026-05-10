//go:build linux

package llm

import (
	"os"
	"strings"
)

func getMachineID() string {
	data, err := os.ReadFile("/etc/machine-id")
	if err != nil {
		return fallbackMachineID()
	}
	return strings.TrimSpace(string(data))
}
