//go:build linux

package namespace

import (
	"fmt"

	"golang.org/x/sys/unix"
)

// MountNamespaceManager manages the mount namespace for the current process. It creates a new mount namespace and
// makes the namespace's mount changes private. This is important because mounting OverlayFS must
// not modify the host's mount namespace. The child process gets its own view of the mount table.
type MountNamespaceManager struct{}

// NewMountNamespaceManager creates a new instance of MountNamespaceManager.
func NewMountNamespaceManager() *MountNamespaceManager {
	return &MountNamespaceManager{}
}

// Setup sets up the mount namespace for the current process. It creates a new mount namespace and
// makes the namespace's mount changes private. This is important because mounting OverlayFS must
// not modify the host's mount namespace. The child process gets its own view of the mount table.
// NB: This should be called BEFORE mounting the overlay filesystem.
func (m *MountNamespaceManager) SetupMountNamespace() error {
	if err := unix.Unshare(unix.CLONE_NEWNS); err != nil {
		return fmt.Errorf("create mount namespace: %w", err)
	}

	// Prevent mount/unmount operations in this namespace from propagating back
	// to the host's mount namespace.
	if err := unix.Mount("", "/", "", unix.MS_REC|unix.MS_PRIVATE, ""); err != nil {
		return fmt.Errorf("make mounts private: %w", err)
	}

	// The overlay is mounted after entering the private mount namespace.
	// This means the mount exists only in this process's namespace.
	return nil

}
