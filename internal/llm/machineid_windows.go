//go:build windows

package llm

import "golang.org/x/sys/windows/registry"

func getMachineID() string {
	key, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Cryptography`, registry.QUERY_VALUE)
	if err != nil {
		return fallbackMachineID()
	}
	defer key.Close()

	guid, _, err := key.GetStringValue("MachineGuid")
	if err != nil {
		return fallbackMachineID()
	}
	return guid
}
