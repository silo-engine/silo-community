package catalogue

import (
	"errors"
	"runtime"
)

// ArchType represents the architecture type of a Silo image.
type ArchType string

const (
	// Arm64 represents the ARM 64-bit architecture.
	Arm64 ArchType = "arm64"
	// Amd64 represents the AMD 64-bit architecture.
	Amd64 ArchType = "amd64"
)


// GetSystemArchType returns the architecture type of the current system.
func GetSystemArchType() (ArchType,error) {
	arch := runtime.GOARCH
	switch arch {
	case "arm64":
		return Arm64,nil
	case "amd64":
		return Amd64,nil
	default:
		return "", errors.New("unsupported architecture: " + arch)
	}
}