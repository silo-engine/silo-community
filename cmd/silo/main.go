//go:build linux

package main

import (
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

// TODO remove the direct instantiation of these subsystems once the top-level SiloApplicationManager is implemented. This will allow for better testability and separation of concerns.
var catalogueManager = catalogue.NewCatalogueManager()
var overlayManager = filesystem.NewOverlayManager(catalogueManager)
var processNamespaceManager = namespace.NewProcessNamespaceManager()
var mountNamespaceManager = namespace.NewMountNamespaceManager()
var shellManager = shell.NewShellManager(processNamespaceManager)

func main() {
	if os.Geteuid() != 0 {
		fmt.Fprintln(os.Stderr, "silo: the application must be run as root")
		fmt.Fprintln(os.Stderr, "usage: sudo ./silo")
		os.Exit(1)
	}

	args := os.Args[1:]
	if len(args) == 0 {
		if err := runTransientShell(); err != nil {
			fatal(err)
		}

		return
	}

	switch args[0] {
	case "create":
		if len(args) != 2 {
			usageAndExit()
		}

		s, err := session.Create(args[1])
		if err != nil {
			fatal(err)
		}

		fmt.Printf("Created Silo environment %q\n", s.Name)
	case "enter":
		if len(args) != 2 {
			usageAndExit()
		}

		s, err := session.Enter(args[1])
		if err != nil {
			fatal(err)
		}
		defer s.Cleanup()

		if err := runShell(s); err != nil {
			fatal(err)
		}
	case "destroy":
		if len(args) != 2 {
			usageAndExit()
		}

		if err := session.Destroy(args[1]); err != nil {
			fatal(err)
		}

		fmt.Printf("Destroyed Silo environment %q\n", args[1])
	default:
		usageAndExit()
	}
}

func runTransientShell() error {
	s, err := session.NewTransient()
	if err != nil {
		return err
	}
	defer s.Cleanup()

	return runShell(s)
}

func runShell(s *session.Session) error {
	cleanup := func() {
		overlayManager.TeardownOverlay(s)
		if s != nil && s.Transient {
			s.Cleanup()
		}
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigCh)
	defer cleanup()

	//if err := namespace.EnterMountNamespace(s); err != nil {
	//	return err
	//}

	//crucially we set up the mount namespace before setting up the overlay, otherwise
	// the overlay will be mounted in the host's mount namespace.
	if err := mountNamespaceManager.SetupMountNamespace(); err != nil {
		return err
	}

	// next setup the filesystem overlay and defer its teardown so that it is cleaned up
	// when the shell exits.
	if err := overlayManager.SetupOverlay(s); err != nil {
		return err
	}
	defer overlayManager.TeardownOverlay(s)

	fmt.Println("Entering Silo...")
	fmt.Println("Changes made inside this shell are isolated in the overlay.")
	fmt.Println("Type 'exit' to leave.")

	//setup a goroutine to listen for termination signals and clean up
	//the overlay and transient session if received.
	go func() {
		<-sigCh
		cleanup()
		os.Exit(1)
	}()

	//if err := namespace.RunShell(s); err != nil {
	//	return err
	//}

	//run the shell in the overlay and wait for it to exit. The shell will be
	// running in a new PID namespace, so it will see itself as PID 1 and can
	// manage its own processes independently of the host system.
	if err := shellManager.RunShell(s); err != nil {
		return err
	}

	return nil
}

func usageAndExit() {
	fmt.Fprintln(os.Stderr, "usage:")
	fmt.Fprintln(os.Stderr, "  silo - creates and runs an ephemeral, isolated environment, the environment (and all changes made within it) are automatically discarded on exit")
	fmt.Fprintln(os.Stderr, "  silo create NAME - create a new named, isolated environment. The environment persists until explicitly destroyed.")
	fmt.Fprintln(os.Stderr, "  silo enter NAME - enter an existing named, isolated environment")
	fmt.Fprintln(os.Stderr, "  silo destroy NAME - destroy an existing named, isolated environment")
	os.Exit(1)
}

func fatal(err error) {
	fmt.Fprintf(os.Stderr, "silo: %v\n", err)
	os.Exit(1)
}
