package llm

import (
	"crypto/sha256"
	"fmt"
	"os"
)

func fallbackMachineID() string {
	hostname, _ := os.Hostname()
	return fmt.Sprintf("%s-%d", hostname, os.Getuid())
}

func deriveEncryptionKey() []byte {
	id := getMachineID()
	hash := sha256.Sum256([]byte(id))
	return hash[:]
}
