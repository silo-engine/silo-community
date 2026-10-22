
 //go:build linux

package namespace

import "syscall"

// ProcessNamespaceManager manages the process namespace for the current process. 
// It creates a new PID namespace for the child process.
type ProcessNamespaceManager struct{}


// NewProcessNamespaceManager creates a new instance of ProcessNamespaceManager.
func NewProcessNamespaceManager() *ProcessNamespaceManager {
	return &ProcessNamespaceManager{}
}

// SetupProcessNamespace configures the child process to use a new PID namespace.
// Linux creates the namespace when the process is started.
func (m *ProcessNamespaceManager) SetupProcessNamespace(attr *syscall.SysProcAttr) {
	attr.Cloneflags |= syscall.CLONE_NEWPID
}
