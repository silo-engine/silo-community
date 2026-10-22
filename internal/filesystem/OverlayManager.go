//go:build linux

// Package filesystem contains the Linux filesystem isolation used by Silo.
package filesystem

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/SharefulNetworks/shareful-utils-silo/internal/catalogue"
	"github.com/SharefulNetworks/shareful-utils-silo/internal/session"
	"golang.org/x/sys/unix"
)

// OverlayManager manages the OverlayFS used for filesystem isolation in Silo.
type OverlayManager struct {
	catalogueManager *catalogue.CatalogueManager
}

// NewOverlayManager creates a new OverlayManager with the given CatalogueManager.
func NewOverlayManager(
	catalogueManager *catalogue.CatalogueManager,
) *OverlayManager {
	return &OverlayManager{
		catalogueManager: catalogueManager,
	}
}

// SetupOverlay mounts an OverlayFS over the session's isolated filesystem.
func (m *OverlayManager) SetupOverlay(s *session.Session) error {
	arch, err := catalogue.GetSystemArchType()
	if err != nil {
		return fmt.Errorf("resolve current CPU architecture: %w", err)
	}

	if arch != catalogue.Arm64 && arch != catalogue.Amd64 {
		return fmt.Errorf(
			"Silo only supports amd64 and arm64 architectures; current architecture is %s",
			arch,
		)
	}

	defaultSiloImage, err := m.catalogueManager.GetBaseImageCatalogueDefaultImage(arch)
	if err != nil {
		return fmt.Errorf("get base image catalogue default image: %w", err)
	}

	defaultSiloImageName := defaultSiloImage.Distro + "-" +
		defaultSiloImage.Version + "--" + string(arch)

	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("get executable path: %w", err)
	}

	resDir := filepath.Join(filepath.Dir(exe), "res")

	lower := filepath.Join(
		resDir,
		"silo-images",
		string(arch),
		defaultSiloImageName+"-release",
		defaultSiloImageName,
	)

	options := fmt.Sprintf(
		"lowerdir=%s,upperdir=%s,workdir=%s",
		lower,
		s.Upper,
		s.Work,
	)

	if err := unix.Mount(
		"overlay",
		s.Merged,
		"overlay",
		0,
		options,
	); err != nil {
		return fmt.Errorf("mount overlay: %w", err)
	}

	if err := NewMountHandler().Apply(s.Merged); err != nil {
		return err
	}

	return nil
}

// TeardownOverlay unmounts the overlay filesystem.
func (m *OverlayManager) TeardownOverlay(s *session.Session) {
	if s == nil || s.Merged == "" {
		return
	}

	//unmount kernel and standard file system mounts
	NewMountHandler().Cleanup(s.Merged)

	//unmount the main file system associated with this session.
	_ = unix.Unmount(s.Merged, unix.MNT_DETACH)
}
