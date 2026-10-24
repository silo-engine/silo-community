package internal

import (
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/silo-engine/silo-community/internal/catalogue"
	"github.com/silo-engine/silo-community/internal/filesystem"
	"github.com/silo-engine/silo-community/internal/namespace"
	"github.com/silo-engine/silo-community/internal/session"
	"github.com/silo-engine/silo-community/internal/shell"
)

// SiloEngine the top-level component that manages the lifecycle of Silo sessions and orchestrates the various subsystems.
// It serves as the main entry point for the Silo application, coordinating the creation, management, and termination
// of Silo sessions.
type SiloEngine struct {
	catalogueManager        *catalogue.CatalogueManager
	overlayManager          *filesystem.OverlayManager
	processNamespaceManager *namespace.ProcessNamespaceManager
	mountNamespaceManager   *namespace.MountNamespaceManager
	shellManager            *shell.ShellManager
}

func NewSiloEngine() *SiloEngine {
	catalogueManager := catalogue.NewCatalogueManager()
	overlayManager := filesystem.NewOverlayManager(catalogueManager)
	processNamespaceManager := namespace.NewProcessNamespaceManager()
	mountNamespaceManager := namespace.NewMountNamespaceManager()
	shellManager := shell.NewShellManager(processNamespaceManager)

	return &SiloEngine{
		catalogueManager:        catalogueManager,
		overlayManager:          overlayManager,
		processNamespaceManager: processNamespaceManager,
		mountNamespaceManager:   mountNamespaceManager,
		shellManager:            shellManager,
	}
}

// StartTransientSession creates and enters a new transient Silo session, which is an isolated environment
// that does not persist any changes made during its lifecycle; any and all changes made within the session
// are discarded upon exit.
func (se *SiloEngine) StartTransientSession() error {

	//create a new transient session and defer its cleanup to ensure that resources are
	// released when the shell exits.
	s, err := session.NewTransient()
	if err != nil {
		return err
	}
	defer s.Cleanup()

	//call helper func to start the silo session.
	return se.startSession(s)
}

// StartPersistentSession enters an existing, persistent Silo session with the specified name,
// which is an isolated environment that retains changes made during its lifecycle; any and all changes
// made within the session are preserved upon exit. If the specified session does not exist, an error is returned.
func (se *SiloEngine) StartPersistentSession(sessionName string) error {

	//loads session with the provided name and defer cleanup to ensure that
	// resources are released when the shell exits.
	s, err := session.Load(sessionName)
	if err != nil {
		return errors.New("An error occurred whislt attempting to enter existing Silo Session: " + err.Error())
	}
	defer s.Cleanup()

	// Acquire lock on the session and defer its release.
	lock, err := session.AcquireSessionLock(sessionName)
	if err != nil {
		return errors.New("An error occurred whilst attempting to acquire a lock on the Silo Session: " + err.Error())
	}
	defer lock.Unlock()

	//call helper func to start the silo session.
	return se.startSession(s)

}

// CreateSession creates a new persistent Silo session with the specified name. The resulting session
// can then be started using the StartPersistentSession method. If a session with the specified name already exists,
// an error is returned, every session name musr be unique.
func (se *SiloEngine) CreateSession(sessionName string) (*session.Session, error) {

	//create a new persistent session with the provided name.
	s, err := session.Create(sessionName)
	if err != nil {
		return nil, errors.New("An error occurred whilst attempting to create a new Silo Session: " + err.Error())
	}
	return s, nil
}

// DestroySession destroys an existing persistent Silo session with the specified name. The session and all its
// associated resources are permanently removed. If the specified session does not exist, an error is returned.
func (se *SiloEngine) DestroySession(sessionName string) error {

	//obtain a lock on the session to ensure that it is not in use by another process
	// before destroying it.
	lock, err := session.AcquireSessionLock(sessionName)
	if err != nil {
		return errors.New("An error occurred whilst attempting to acquire a lock on the Silo Session: " + err.Error() + " please ensure that the session is is not in use by any processes before attempting to destroy it.")
	}
	defer lock.Unlock()

	//destroy the persistent session with the provided name.
	if err := session.Destroy(sessionName); err != nil {
		return errors.New("An error occurred whilst attempting to destroy the Silo Session: " + err.Error())
	}
	return nil
}

//Helpers....

// startSession is a helper method that encapsulates the common logic for starting both transient and persistent Silo sessions.
func (se *SiloEngine) startSession(s *session.Session) error {
	cleanup := func() {
		se.overlayManager.TeardownOverlay(s)
		if s != nil && s.Transient {
			s.Cleanup()
		}
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigCh)
	defer cleanup()

	//crucially we set up the mount namespace before setting up the overlay, otherwise
	// the overlay will be mounted in the host's mount namespace.
	if err := se.mountNamespaceManager.SetupMountNamespace(); err != nil {
		return err
	}

	// next setup the filesystem overlay and defer its teardown so that it is cleaned up
	// when the shell exits.
	if err := se.overlayManager.SetupOverlay(s); err != nil {
		return err
	}
	defer se.overlayManager.TeardownOverlay(s)

	fmt.Println("Entering Silo...")
	fmt.Println("Changes made inside this shell are isolated from the host system.")
	fmt.Println("Type 'exit' to leave.")

	//setup a goroutine to listen for termination signals and clean up
	//the overlay and transient session if received.
	go func() {
		<-sigCh
		cleanup()
		os.Exit(1)
	}()

	//run the shell in the overlay and wait for it to exit. The shell will be
	// running in a new PID namespace, so it will see itself as PID 1 and can
	// manage its own processes independently of the host system.
	if err := se.shellManager.RunShell(s); err != nil {
		fmt.Fprintf(os.Stderr, "SILO: An error occurred whilst running the Silo shell: %v\n", err)
	}

	return nil

}
