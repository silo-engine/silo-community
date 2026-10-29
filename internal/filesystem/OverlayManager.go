//go:build linux

// Package filesystem contains the Linux filesystem isolation used by Silo.
package filesystem

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/silo-engine/silo-community/internal/session"
	"golang.org/x/sys/unix"
)

// OverlayManager manages the OverlayFS used for filesystem isolation in Silo.
type OverlayManager struct {
}

// NewOverlayManager creates a new OverlayManager with the given CatalogueManager.
func NewOverlayManager() *OverlayManager {
	return &OverlayManager{}
}

// SetupOverlay mounts an OverlayFS over the session's isolated filesystem.
func (m *OverlayManager) SetupOverlay(session *session.SiloSession) error {

	//grab reference to the session's base image metadata
	sessionBaseImage := session.Metadata.BaseImage

	//build base image root release directory name from metadata.
	sessionBaseImageName := sessionBaseImage.Distro + "-" +
		sessionBaseImage.Version + "--" + sessionBaseImage.ArchString

	//build path to the top-level resources directory, which contains the Silo images.
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("get executable path: %w", err)
	}
	resDir := filepath.Join(filepath.Dir(exe), "res")

	//build path to the lower directory of the overlay filesystem, which is the base image release directory.
	lower := filepath.Join(
		resDir,
		"silo-images",
		sessionBaseImage.ArchString,
		sessionBaseImageName+"-release",
		sessionBaseImageName,
	)

	//check that the lower directory exists, if not return an error. Generally the path should
	//exist, providing it was not explicitly deleted by the user,or the session was imported
	//from a different system that does not have the same base image installed.
	if _, err := os.Stat(lower); os.IsNotExist(err) {
		return fmt.Errorf("specified base image release directory does not exist: %s", lower)
	}

	// validate that the base image checksum, as specified in the session's metadata,
	// matches the checksum of the base image release. The checksum.chk is in the root release
	//directory (i.e. the parent directory of the lower directory).
	baseImageReleaseDir := filepath.Dir(lower)
	if err := m.validateBaseImageReleaseChecksum(session, baseImageReleaseDir); err != nil {
		return fmt.Errorf("validate base image release checksum: %w", err)
	}

	options := fmt.Sprintf(
		"lowerdir=%s,upperdir=%s,workdir=%s",
		lower,
		session.Upper,
		session.Work,
	)

	if err := unix.Mount(
		"overlay",
		session.Merged,
		"overlay",
		0,
		options,
	); err != nil {
		return fmt.Errorf("mount overlay: %w", err)
	}

	if err := NewMountHandler().Apply(session.Merged); err != nil {
		return err
	}

	return nil
}

// TeardownOverlay unmounts the overlay filesystem.
func (m *OverlayManager) TeardownOverlay(s *session.SiloSession) {
	if s == nil || s.Merged == "" {
		return
	}

	//unmount kernel and standard file system mounts
	NewMountHandler().Cleanup(s.Merged)

	//unmount the main file system associated with this session.
	_ = unix.Unmount(s.Merged, unix.MNT_DETACH)
}

//helpers..

func (m *OverlayManager) validateBaseImageReleaseChecksum(session *session.SiloSession, baseReleaseDir string) error {

	checksumFile := filepath.Join(baseReleaseDir, "checksum.chk")

	//read the first and only line of the checksum file, which contains the checksum of this base image release
	checksumBytes, err := os.ReadFile(checksumFile)
	if err != nil {
		return fmt.Errorf("read checksum file: %w", err)
	}
	checksum := string(checksumBytes)
	checksum = strings.TrimSpace(checksum) //strip any newlines from the checksum string

	//compare the checksum read from the file with the checksum specified in the session's metadata.
	if checksum != session.Metadata.BaseImage.ReleaseChecksum {
		return fmt.Errorf("base image release checksum mismatch: expected %s, got %s this indicates that the session you are attempting to load requires a different version of the specified base image. You can download the correct base image version for this session by running: \n silo images pull --id %s ", session.Metadata.BaseImage.ReleaseChecksum, checksum, session.Metadata.BaseImage.ReleaseVersion)
	}

	return nil
}
