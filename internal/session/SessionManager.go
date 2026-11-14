package session

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gofrs/flock"
	"github.com/silo-engine/silo-community/internal/catalogue"
	"github.com/silo-engine/silo-community/internal/common"
)

const NAMED_ENVIRONMENT_ROOT = "/var/lib/silo/envs"
const TRANSIENT_ENVIRONMENT_ROOT = "/var/lib/silo/tenvs"
const SESSION_LOCK_FILE = "session.lock"

// SessionManager is responsible for managing Silo sessions, including creating, loading, and destroying them.
// It maintains a registry of all sessions and provides methods to manage their respective lifecycles.
type SessionManager struct {
	catalogueManager *catalogue.CatalogueManager
}

func NewSessionManager(catalogueManager *catalogue.CatalogueManager) *SessionManager {
	return &SessionManager{
		catalogueManager: catalogueManager,
	}
}

// NewTransient creates the directory layout for a new disposable Silo session.
func (sm *SessionManager) NewTransient(imageIdOrName string) (*SiloSession, error) {

	if err := os.MkdirAll(TRANSIENT_ENVIRONMENT_ROOT, 0755); err != nil {
		return nil, fmt.Errorf("create transient environment root: %w", err)
	}
	root, err := os.MkdirTemp(TRANSIENT_ENVIRONMENT_ROOT, "temp-")
	if err != nil {
		return nil, fmt.Errorf("create transient session directory: %w", err)
	}

	//create and return a new SiloSession object, that represents the active session, to return to the caller.
	//the caller is responsible for calling the Cleanup() method on the returned SiloSession object
	// to dispose of the session and clean up its resources when it is no longer needed.
	return sm.newSession(root, true, "", true, imageIdOrName)
}

// NewPersistent creates a persistent named environment under the well-known base path.
func (sm *SessionManager) NewPersistent(name string, imageidOrName string) (*SiloSession, error) {
	if err := sm.validateName(name); err != nil {
		return nil, err
	}

	if err := os.MkdirAll(NAMED_ENVIRONMENT_ROOT, 0755); err != nil {
		return nil, fmt.Errorf("create environment root: %w", err)
	}

	root := filepath.Join(NAMED_ENVIRONMENT_ROOT, name)
	if err := os.Mkdir(root, 0755); err != nil {
		if os.IsExist(err) {
			return nil, fmt.Errorf("environment %q already exists", name)
		}

		return nil, fmt.Errorf("create environment %q: %w", name, err)
	}

	//create the new SiloSession object, that represents the active session, to return to the caller.
	//the caller is responsible for calling the Cleanup() method on the returned SiloSession object
	// to dispose of the session and clean up its resources when it is no longer needed.
	sess, err := sm.newSession(root, false, name, true, imageidOrName)
	if err != nil {
		//cleanup the environment directory if session creation fails.
		_ = os.RemoveAll(root)
		return nil, fmt.Errorf("create persistent session: %w", err)
	}

	//serialise the session's metadata to a file in the session's root directory for future reference.
	//this will allows the exact base image used to create the session to be perminently recorded and associated
	// with the session for future reference.
	if err := sm.serializeToFile(filepath.Join(sess.Root, "metadata.json"), sess.Metadata); err != nil {
		return nil, fmt.Errorf("failed to serialize session metadata to file: %w", err)
	}

	return sess, nil
}

// LoadMeta loads the metadata for a persistent named environment without loading the full session.
func (sm *SessionManager) LoadMeta(name string) (*Metadata, error) {
	if err := sm.validateName(name); err != nil {
		return nil, err
	}

	root := filepath.Join(NAMED_ENVIRONMENT_ROOT, name)
	info, err := os.Stat(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("environment/session: %q does not exist", name)
		}

		return nil, fmt.Errorf("unable to open environment/session %q: %w", name, err)
	}

	if !info.IsDir() {
		return nil, fmt.Errorf("environment/session %q is not a directory", name)
	}

	// Load the metadata file for the environment.
	metadata, err := sm.deserializeFromFile(filepath.Join(root, "metadata.json"))
	if err != nil {
		return nil, fmt.Errorf("load environment/session metadata: %w", err)
	}

	return metadata, nil
}

// Load loads an existing persistent named environment.
func (sm *SessionManager) Load(name string) (*SiloSession, error) {
	if err := sm.validateName(name); err != nil {
		return nil, err
	}

	root := filepath.Join(NAMED_ENVIRONMENT_ROOT, name)
	info, err := os.Stat(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("environment %q does not exist", name)
		}

		return nil, fmt.Errorf("open environment %q: %w", name, err)
	}

	if !info.IsDir() {
		return nil, fmt.Errorf("environment %q is not a directory", name)
	}

	// Load metadata first so we can validate and construct the session against the
	// original base image this environment was created with.
	metadata, err := sm.deserializeFromFile(filepath.Join(root, "metadata.json"))
	if err != nil {
		return nil, fmt.Errorf("load session metadata: %w", err)
	}

	if metadata == nil || metadata.BaseImage == nil {
		return nil, fmt.Errorf("load session metadata: missing base image metadata")
	}

	imageIdOrName := metadata.BaseImage.ReleaseVersion
	if imageIdOrName == "" {
		if metadata.BaseImage.Distro == "" || metadata.BaseImage.Version == "" {
			return nil, fmt.Errorf("load session metadata: missing base image identity")
		}
		imageIdOrName = metadata.BaseImage.Distro + "-" + metadata.BaseImage.Version
	}

	loadedSession, err := sm.newSession(root, false, name, false, imageIdOrName)
	if err != nil {
		return nil, fmt.Errorf("Failed to load session: %q: %w", name, err)
	}

	// Preserve the original persisted metadata on the loaded session.
	loadedSession.Metadata = metadata

	return loadedSession, nil
}

// Destroy removes a persistent named environment.
func (sm *SessionManager) Destroy(name string) error {
	if err := sm.validateName(name); err != nil {
		return err
	}

	return os.RemoveAll(filepath.Join(NAMED_ENVIRONMENT_ROOT, name))
}

// List lists all existing persistent named environments/sessions metadata
func (sm *SessionManager) List() ([]*Metadata, error) {

	entries, err := os.ReadDir(NAMED_ENVIRONMENT_ROOT)
	if err != nil {
		return nil, fmt.Errorf("failed to read environment root: %w", err)
	}

	var sessions []*Metadata
	for _, entry := range entries {
		if entry.IsDir() {
			metadata, err := sm.LoadMeta(entry.Name())
			if err != nil {
				return nil, fmt.Errorf("load environment/session %q: %w", entry.Name(), err)
			}

			lock, err := sm.AcquireSessionLock(entry.Name())
			if err != nil {
				if strings.Contains(err.Error(), "session is already active") {
					metadata.Active = true
				} else {
					return nil, fmt.Errorf("determine active state for %q: %w", entry.Name(), err)
				}
			} else {
				metadata.Active = false
				if err := lock.Unlock(); err != nil {
					return nil, fmt.Errorf("release session lock for %q: %w", entry.Name(), err)
				}
			}

			sessions = append(sessions, metadata)

			// sort sessions by active state
			sort.Slice(sessions, func(i, j int) bool {
				if sessions[i].Active && !sessions[j].Active {
					return true
				}
				if !sessions[i].Active && sessions[j].Active {
					return false
				}
				return sessions[i].CreatedAt > sessions[j].CreatedAt
			})
		}
	}

	return sessions, nil
}

// Export exports a session to a file.
func (sm *SessionManager) Export(name string, outputPath string) error {
	if err := sm.validateName(name); err != nil {
		return err
	}

	//ensure outputPath exists and is a directory.
	info, err := os.Stat(outputPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("output path %q does not exist", outputPath)
		}
		return fmt.Errorf("failed to stat output path %q: %w", outputPath, err)
	}

	if !info.IsDir() {
		return fmt.Errorf("output path %q is not a directory", outputPath)
	}

	// sessions can ONLY be exported if they are not currently active, as we cannot
	// guarantee the integrity of the session data if it is being modified while we are exporting it.
	// Attempt to acquire a lock on the session to check if it is active.
	lock, err := sm.AcquireSessionLock(name)
	if err != nil {
		if strings.Contains(err.Error(), "session is already active") {
			return fmt.Errorf("cannot export session %q: session is currently active", name)
		}
		return fmt.Errorf("failed to acquire lock for session %q: %w", name, err)
	}
	defer func() {
		if err := lock.Unlock(); err != nil {
			fmt.Printf("warning: failed to release lock for session %q: %v\n", name, err)
		}
	}()

	//create a temporary directory to build out export package including the upper layer and metadata.
	tempExportDir, err := os.MkdirTemp("", "silo-session-export-*")
	if err != nil {
		return fmt.Errorf("failed to create temporary export directory: %w", err)
	}
	defer os.RemoveAll(tempExportDir)

	//load the session to be exported.
	session, err := sm.Load(name)
	if err != nil {
		return fmt.Errorf("failed to load session %q: %w", name, err)
	}

	//export the upper layer of the session to as the (session filesystem) tar archive in the temporary export directory.
	exportedFileSystemPath := filepath.Join(tempExportDir, "filesystem.tar")
	if err := exportSessionUpperLayer(session.Upper, exportedFileSystemPath); err != nil {
		return fmt.Errorf("failed to export session upper layer for session %q: %w", name, err)
	}

	//compute the checksum of the exported filesystem tar archive to include in the metadata for integrity verification.
	checksum, err := common.ComputeChecksum(exportedFileSystemPath)
	if err != nil {
		return fmt.Errorf("failed to compute checksum for exported filesystem: %w", err)
	}

	//store the checksum to the temp export directory as a separate file for integrity verification.
	checksumFilePath := filepath.Join(tempExportDir, "filesystem.chk")
	if err := os.WriteFile(checksumFilePath, []byte(checksum), 0644); err != nil {
		return fmt.Errorf("failed to write checksum file: %w", err)
	}

	//store the session metadata to the temp export directory as a separate file.
	metadataFilePath := filepath.Join(tempExportDir, "metadata.json")
	if err := sm.serializeToFile(metadataFilePath, session.Metadata); err != nil {
		return fmt.Errorf("failed to serialize session metadata to file: %w", err)
	}

	//create a zip archive of the temp export directory containing the exported filesystem, checksum, and metadata.
	// the outputpath will just be a directory thus we will want to append the name.sesh to the output path to
	// create the final output file path.
	fullOutputPath := filepath.Join(outputPath, name+".sesh")
	zipCmd := exec.Command("zip", "-r", fullOutputPath, ".")
	zipCmd.Dir = tempExportDir
	if output, err := zipCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to create %s.sesh (Silo Environment Shapshot Handle) file: %w: %s", fullOutputPath, err, output)
	}

	return nil
}

func (sm *SessionManager) Import(filePath string, name string) error {
	if err := sm.validateName(name); err != nil {
		return err
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read session file: %w", err)
	}

	var session SiloSession
	if err := json.Unmarshal(data, &session); err != nil {
		return fmt.Errorf("failed to unmarshal session: %w", err)
	}

	// Set the session's root directory to the new environment's root.
	session.Root = filepath.Join(NAMED_ENVIRONMENT_ROOT, name)

	// Ensure the session's root directory is set correctly for the new environment.
	if err := os.MkdirAll(session.Root, 0755); err != nil {
		return fmt.Errorf("failed to create session root: %w", err)
	}

	return nil
}

func (sm *SessionManager) AcquireSessionLock(name string) (*flock.Flock, error) {
	envPath := filepath.Join(NAMED_ENVIRONMENT_ROOT, name)
	lockPath := filepath.Join(envPath, SESSION_LOCK_FILE)
	fileLock := flock.New(lockPath)

	locked, err := fileLock.TryLock()
	if err != nil {
		return nil, fmt.Errorf("acquire session lock: %w", err)
	}

	if !locked {
		return nil, errors.New("session is already active")
	}

	return fileLock, nil
}

func (sm *SessionManager) newSession(
	root string,
	transient bool,
	name string,
	cleanupOnError bool,
	imageNameOrId string) (*SiloSession, error) {

	//create the new SiloSession object, that represents the active session, to return to the caller.
	s := &SiloSession{
		Root:      root,
		Upper:     filepath.Join(root, "upper"),
		Work:      filepath.Join(root, "work"),
		Merged:    filepath.Join(root, "merged"),
		Transient: transient,
		Name:      name,
	}

	//if an optional image name or id was provided, attempt to find the image in the catalogue and associate it with the session.
	if imageNameOrId != "" {
		image, found := sm.catalogueManager.GetCatalogueImage(imageNameOrId)
		if found {

			//create and populate the relevant field in the sessions Metadata struct with the image details.
			baseImageMetaData := NewBaseImageMetadata(
				image.Distro,
				image.Version,
				image.ReleaseVersion,
				image.ReleaseChecksum,
				image.Description,
				image.ImageURL,
				image.GetArchString(),
			)

			// populate the session's metadata with the base image details.
			s.Metadata = NewMetadata(baseImageMetaData,
				name,
				time.Now().UTC().String())

		} else {
			return nil, fmt.Errorf("failed to find specified image %q in the catalogue", imageNameOrId)
		}
	} else {
		//otherwise if no image was specified, attempt to find the default image for the current system architecture and associate it with the session.
		defaultImage, error := sm.catalogueManager.GetBaseImageCatalogueDefaultImage()
		if error != nil {
			return nil, fmt.Errorf("failed to retrieve default image for the current system architecture: %w", error)
		}

		//as the default image struct contains partial info about the image, i.e. excluding release checksum we use
		//the "release version" of the default image to find the full image details in the catalogue and associate it with the session.
		image, found := sm.catalogueManager.GetCatalogueImage(defaultImage.ReleaseVersion)
		if !found {
			return nil, fmt.Errorf("failed to find default image %q in the catalogue", defaultImage.ReleaseVersion)
		}

		//create and populate the relevant field in the sessions Metadata struct with the default image details.
		baseImageMetaData := NewBaseImageMetadata(
			image.Distro,
			image.Version,
			image.ReleaseVersion,
			image.ReleaseChecksum,
			image.Description,
			image.ImageURL,
			image.GetArchString(),
		)

		// populate the session's metadata with the base image details.
		s.Metadata = NewMetadata(baseImageMetaData,
			name,
			time.Now().UTC().String())
	}

	// next ensure that the base image release directory exists for the specified base image,
	imageNameVersionAndArch := s.Metadata.BaseImage.Distro + "-" + s.Metadata.BaseImage.Version + "--" + s.Metadata.BaseImage.ArchString
	imageNameAndVersion := s.Metadata.BaseImage.Distro + "-" + s.Metadata.BaseImage.Version
	if !sm.baseImageReleaseDirExists(imageNameVersionAndArch, s.Metadata.BaseImage.ArchString) {
		return nil, fmt.Errorf("The specified base image: %q exists in the catalogue, but DOES NOT appear to have been installed. You can install it using: \n 'silo images pull --name %s' ", imageNameAndVersion, imageNameAndVersion)
	}

	//setup the directory structure for the new session, including the upper, work, and merged directories.
	for _, dir := range []string{s.Upper, s.Work, s.Merged} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			if cleanupOnError {
				_ = os.RemoveAll(root)
			}

			return nil, fmt.Errorf("create %s: %w", dir, err)
		}
	}

	return s, nil
}

// Cleanup removes the temporary session directory.
func (s *SiloSession) Cleanup() {

	if s == nil || !s.Transient || s.Root == "" {
		return
	}

	//fmt.Printf("Cleaning up transient Silo session %q\n", s.Root)
	_ = os.RemoveAll(s.Root)
}

//helpers

// validateName checks if the provided environment name is valid.
// It ensures that the name is not empty and does not contain any invalid
// characters or path components.
func (sm *SessionManager) validateName(name string) error {
	if name == "" {
		return fmt.Errorf("environment name cannot be empty")
	}

	if filepath.Base(name) != name || name == "." || name == ".." {
		return fmt.Errorf("invalid environment name %q", name)
	}

	return nil
}

// serializeToFile serializes the provided Metadata to a JSON file at the specified filePath.
func (sm *SessionManager) serializeToFile(filePath string, metadata *Metadata) error {
	if metadata == nil {
		return fmt.Errorf("metadata is nil")
	}

	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("create metadata file: %w", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(metadata); err != nil {
		return fmt.Errorf("encode metadata to JSON: %w", err)
	}

	return nil
}

func (sm *SessionManager) deserializeFromFile(filePath string) (*Metadata, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("open metadata file: %w", err)
	}
	defer file.Close()

	var metadata Metadata
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&metadata); err != nil {
		return nil, fmt.Errorf("decode metadata from JSON: %w", err)
	}

	return &metadata, nil
}

// baseImageReleaseDirExists checks if the base image release directory exists for the
// specified base image name. which will the the following form [image]-[version].
func (sm *SessionManager) baseImageReleaseDirExists(baseImageName string, archString string) bool {

	//build path to the top-level resources directory, which contains the Silo images.
	exe, err := os.Executable()
	if err != nil {
		return false
	}
	resDir := filepath.Join(filepath.Dir(exe), "res")

	//full path to image
	imagePath := filepath.Join(
		resDir,
		"silo-images",
		archString,
		baseImageName+"-release",
		baseImageName,
	)

	//if the image path exists, return true, otherwise return false.
	if _, err := os.Stat(imagePath); os.IsNotExist(err) {
		return false
	}

	return true

}

func exportSessionUpperLayer(upperDir, archivePath string) error {
	cmd := exec.Command(
		"tar",
		"--create",
		"--file="+archivePath,
		"--directory="+upperDir,
		"--numeric-owner",
		"--xattrs",
		"--xattrs-include=*",
		"--acls",
		"--selinux",
		".",
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("export upper layer: %w: %s", err, output)
	}

	return nil
}
