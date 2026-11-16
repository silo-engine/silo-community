package common

// ArchType represents the architecture type of a Silo image.
type ArchType string

const (
	// Arm64 represents the ARM 64-bit architecture.
	Arm64 ArchType = "arm64"
	// Amd64 represents the AMD 64-bit architecture.
	Amd64 ArchType = "amd64"
)
