package session

// SiloSession represents a Silo session, which is an isolated environment that can be either persistent or transient.
// A persistent session retains its state across invocations, while a transient session discards any changes made
// during its lifecycle.
type SiloSession struct {
	// Root is the directory containing the session's writable state.
	Root string

	// Upper is the OverlayFS upper directory. Writes made by the shell go here.
	Upper string

	// Work is required by OverlayFS and must be on the same filesystem as Upper.
	Work string

	// Merged is the directory where the overlay filesystem is mounted.
	Merged string

	// Transient indicates whether the session should be removed on cleanup.
	Transient bool

	// Name is the persistent environment name, if any.
	Name string

	// Metadata contains additional information about the session.
	Metadata *Metadata
}
