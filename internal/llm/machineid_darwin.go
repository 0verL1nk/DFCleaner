//go:build darwin

package llm

import (
	"os/exec"
	"strings"
)

func getMachineID() string {
	out, err := exec.Command("ioreg", "-rd1", "-c", "IOPlatformExpertDevice").Output()
	if err != nil {
		return fallbackMachineID()
	}
	for _, line := range strings.Split(string(out), "\n") {
		if strings.Contains(line, "IOPlatformUUID") {
			parts := strings.Split(line, `"`)
			if len(parts) >= 4 {
				return parts[len(parts)-2]
			}
		}
	}
	return fallbackMachineID()
}
