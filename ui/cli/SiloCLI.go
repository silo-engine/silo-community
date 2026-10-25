package cli

import (
	"fmt"
	"os"

	"github.com/silo-engine/silo-community/internal"
)

// SiloCLI is the command-line interface for the Silo application, providing users with a way to
// interact with Silo sessions and manage isolated environments through terminal commands.
type SiloCLI struct {
	siloEngine *internal.SiloEngine
}

// NewSiloCLI creates a new instance of SiloCLI.
func NewSiloCLI() *SiloCLI {
	return &SiloCLI{
		siloEngine: internal.NewSiloEngine(),
	}
}

// Run starts the SiloCLI, processing command-line arguments and executing the corresponding actions.
func (scli *SiloCLI) Run() {
	if os.Geteuid() != 0 {
		fmt.Fprintln(os.Stderr, "silo: the application must be run as root")
		fmt.Fprintln(os.Stderr, "usage: sudo ./silo")
		os.Exit(1)
	}

	args := os.Args[1:]
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "silo: no command provided")
		usageAndExit()
	}

	switch args[0] {
	case "create":
		if len(args) != 2 {
			usageAndExit()
		}

		s, err := scli.siloEngine.CreateSession(args[1])
		if err != nil {
			fatal(err)
		}
		fmt.Printf("Created Silo session/environment %q\n", s.Name)
	case "enter":

		//if no values was provided with create start a transient session, otherwise start a
		// persistent session with the provided name.
		if len(args) == 1 {
			fmt.Fprintln(os.Stderr, "silo: no session name provided, entering a transient session..")
			if err := scli.siloEngine.StartTransientSession(); err != nil {
				fatal(err)
			}
		} else if len(args) == 2 {
			fmt.Fprintln(os.Stderr, "silo: entering session: ", args[1])
			if err := scli.siloEngine.StartPersistentSession(args[1]); err != nil {
				fatal(err)
			}
		} else if len(args) == 3 && args[2] == "--create" {
			fmt.Fprintln(os.Stderr, "silo: creating and entering session: ", args[1])
			if _, err := scli.siloEngine.CreateSession(args[1]); err != nil {
				fatal(err)
			}
			if err := scli.siloEngine.StartPersistentSession(args[1]); err != nil {
				fatal(err)
			}

		} else {
			usageAndExit()
		}
	case "destroy":
		if len(args) != 2 {
			usageAndExit()
		}

		if err := scli.siloEngine.DestroySession(args[1]); err != nil {
			fatal(err)
		}

		fmt.Printf("Destroyed Silo session/environment %q\n", args[1])
	case "images":
		if len(args) == 1 {
			fmt.Fprintln(os.Stderr, "silo: no image operation provided")
			usageAndExit()
		} else {

			switch args[1] {
			case "list":
				//if err := scli.siloEngine.ListBaseImages(); err != nil {
				//	fatal(err)
				//}
			case "update":
				//if err := scli.siloEngine.UpdateBaseImages(); err != nil {
				//	fatal(err)
				//}
			case "pull":

				//arg 2 should be the id or name of the image to pull, if not provided print usage and exit.
				if len(args) < 3 {
					fmt.Fprintln(os.Stderr, "silo: no image name provided for pull operation")
					usageAndExit()
				}
				imageIdOrName := args[2]

				//next search for the optional --catalogue flag and its value, if provided.
				catalogueName := ""
				for i, arg := range args {
					if arg == "--catalogue" && i+1 < len(args) {
						catalogueName = args[i+1]
						break
					}
				}

				if err := scli.siloEngine.DownloadCatalogueImage(catalogueName, imageIdOrName); err != nil {
					fatal(err)
				}

			default:
				fmt.Fprintln(os.Stderr, "silo: unknown image operation provided")
				usageAndExit()
			}

		}

	default:
		usageAndExit()
	}

}

//helper functions

// usageAndExit helper function prints the usage information for the SiloCLI and exits
// the program with a non-zero status code.
func usageAndExit() {
	fmt.Fprintln(os.Stderr, "usage:")
	fmt.Fprintln(os.Stderr, "  silo - displays this usage information")
	fmt.Fprintln(os.Stderr, "  silo enter - creates and enters an ephemeral, session/isolated environment, the environment (and all changes made within it) are automatically discarded on exit")
	fmt.Fprintln(os.Stderr, "  silo enter [<NAME>] [--create [<NAME>]] - enters an existing named, session/isolated environment. Where the --create [<NAME>] option is provided, a new named environment is created and entered. ")
	fmt.Fprintln(os.Stderr, "  silo create [<NAME>] - create a new named, isolated environment. The environment persists until explicitly destroyed.")
	fmt.Fprintln(os.Stderr, "  silo destroy [<NAME>] - destroy an existing named, isolated environment")
	os.Exit(1)
}

// fatal helper function prints an error message to standard error and exits the program with a non-zero status code.
func fatal(err error) {
	fmt.Fprintf(os.Stderr, "silo: %v\n", err)
	os.Exit(1)
}
