//go:build linux

package shell

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/silo-engine/silo-community/internal/namespace"
	"github.com/silo-engine/silo-community/internal/session"
)

// ShellManager manages the launching of an interactive shell within a Silo session.
type ShellManager struct {
	processNamespaceManager *namespace.ProcessNamespaceManager
	mountNamespaceManager   *namespace.MountNamespaceManager
}

// NewShellManager creates a new instance of ShellManager with the given ProcessNamespaceManager.
func NewShellManager(
	processNamespaceManager *namespace.ProcessNamespaceManager,
) *ShellManager {
	return &ShellManager{
		processNamespaceManager: processNamespaceManager,
	}
}

func (m *ShellManager) RunShell(s *session.SiloSession) error {
	promptName := s.Name

	if s.Transient {
		if promptName == "" {
			promptName = filepath.Base(s.Root)
		}
	} else if promptName == "" {
		promptName = "unnamed"
	}

	// Clear the terminal to ensure a clean display of the prompt.
	fmt.Print("\033[H\033[2J")

	// Prepare the bootstrap script that sets up the shell environment and executes the shell.
	bootstrap := fmt.Sprintf(
		"export PS1=%s; mount -t proc proc /proc && exec /bin/bash --noprofile --norc -i",
		shellQuote("\\[\\e[92m\\]silo#"+promptName+"> \\[\\e[0m\\] "),
	)

	// Create the command to run the child process shell with the bootstrap script.
	cmd := exec.Command("/bin/bash", "-c", bootstrap)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Dir = "/"

	// Set up the SysProcAttr to configure the child process's namespaces.
	attr := &syscall.SysProcAttr{
		Chroot: s.Merged,
	}
	m.processNamespaceManager.SetupProcessNamespace(attr)
	cmd.SysProcAttr = attr

	// Prepare the environment variables for the child process.
	env := make([]string, 0, len(os.Environ())+2)
	for _, entry := range os.Environ() {
		if strings.HasPrefix(entry, "PROMPT_COMMAND=") {
			continue
		}
		env = append(env, entry)
	}

	cmd.Env = append(env,
		"SILO_AUTH=QXBwOiBTaWxvCkRlc2M6IEluc3RhbnQsIElzb2xhdGVkLCBMaW51eCBFbnZpcm9ubWVudHMKQXV0aG9yOiBHaWxlcyBUaG9tcHNvbgpEYXRlOiAxMS8xMC8yMDI2CkNvcHlyaWdodDogwqkgMjAyNiBHaWxlcyBUaG9tcHNvbi4gQWxsIFJpZ2h0cyBSZXNlcnZlZA==",
		"SILO_ROOT=/",
	)

	// Run the child process shell and wait for it to complete.
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("shell: %w", err)
	}

	return nil
}

// shellQuote safely quotes a string for use in a shell command.
func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}
