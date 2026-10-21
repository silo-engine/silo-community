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

// SetupOverlay mounts an OverlayFS over the host root filesystem.
// The host filesystem becomes the read-only lower layer. Any writes made
// inside Silo are redirected to the session's temporary upper layer.
func SetupOverlay(s *session.Session) error {

	//get the current architecture of the host system
	arch, err := catalogue.GetSystemArchType()
	if err != nil {
		return fmt.Errorf("An error occurred while attempting to resolve the current CPU architecture: %w", err)
	}
	if arch != catalogue.Arm64 && arch != catalogue.Amd64 {
		return fmt.Errorf("Silo only supports amd64 and arm64 architectures. The current architecture is %s", arch)
	}
		

	//instantiate a new CatalogueManager to load the silo image catalogues from JSON files in the res/silo-images/catalogues directory
	// we use this subsystem to obtain reference to the default base image details for the current architecture.
	catalogueManager := catalogue.NewCatalogueManager()
	defaultSiloImage, err := catalogueManager.GetBaseImageCatalogueDefaultImage(arch)
	if err != nil {
		return fmt.Errorf("get base image catalogue default image: %w", err)
	}

	//build default image name
	defaultSiloImageName := defaultSiloImage.Distro + "-" + defaultSiloImage.Version + "--" + string(arch);

	// The overlay's lower layer is now a minimal filesystem that contains only the
	// files necessary to run a shell. This is done to avoid polluting the overlay
	// with the host's root file system
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("get executable path: %w", err)
	}

	resDir := filepath.Join(filepath.Dir(exe), "res")

	lower := filepath.Join(
		resDir,
		"silo-images",
		"arm64",
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

	// setup requisite mounts inside the overlay so that the shell can see /proc, /dev, etc.
	if err := NewDefaultMountHandler().Apply(s.Merged); err != nil {
		return err
	}

	return nil
}

// injectDNS copies the host's /etc/resolv.conf into the overlay's upper layer so that DNS resolution works inside Silo.
func injectDNS(upperDir string) error {
	sourcePath := "/etc/resolv.conf"
	destinationPath := filepath.Join(upperDir, "etc", "resolv.conf")

	contents, err := os.ReadFile(sourcePath)
	if err != nil {
		return fmt.Errorf("read %s: %w", sourcePath, err)
	}

	if err := os.MkdirAll(filepath.Dir(destinationPath), 0755); err != nil {
		return fmt.Errorf("create DNS directory: %w", err)
	}

	if err := os.WriteFile(destinationPath, contents, 0644); err != nil {
		return fmt.Errorf("write %s: %w", destinationPath, err)
	}

	return nil
}

// TeardownOverlay unmounts the overlay filesystem.
//
// MNT_DETACH allows teardown to proceed even if a process still has a reference
// to the mount. The shell should normally have exited before this is called.
func TeardownOverlay(s *session.Session) {
	if s == nil || s.Merged == "" {
		return
	}

	NewDefaultMountHandler().Cleanup(s.Merged)
	_ = unix.Unmount(s.Merged, unix.MNT_DETACH)
}
