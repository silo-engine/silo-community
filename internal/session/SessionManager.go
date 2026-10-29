package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/gofrs/flock"
	"github.com/silo-engine/silo-community/internal/catalogue"
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

	loadedSession, err := sm.newSession(root, false, name, false, "")
	if err != nil {
		return nil, fmt.Errorf("load environment %q: %w", name, err)
	}

	//finally load the sessions metadata which will crucially include the details of the base image
	// used to create the session, from the metadata.json file in the session's root directory and
	// associate it with the loaded session.
	metadata, err := sm.deserializeFromFile(filepath.Join(loadedSession.Root, "metadata.json"))
	if err != nil {
		return nil, fmt.Errorf("load session metadata: %w", err)
	}
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

func (sm *SessionManager) AcquireSessionLock(name string) (*flock.Flock, error) {
	envPath := filepath.Join(NAMED_ENVIRONMENT_ROOT, name)
	lockPath := filepath.Join(envPath, "session.lock")
	fileLock := flock.New(lockPath)

	locked, err := fileLock.TryLock()
	if err != nil {
		return nil, fmt.Errorf("acquire session lock: %w", err)
	}

	if !locked {
		return nil, fmt.Errorf("session is already active")
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
			s.Metadata = NewMetadata(baseImageMetaData)

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
		s.Metadata = NewMetadata(baseImageMetaData)
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
