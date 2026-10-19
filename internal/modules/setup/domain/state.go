// Package domain contains system-first-initialization business language.
package domain

// InstallationState reports whether system first initialization has completed.
type InstallationState struct {
	Initialized bool
}

// NewInstallationState creates an installation-state value.
func NewInstallationState(initialized bool) InstallationState {
	return InstallationState{Initialized: initialized}
}
