// Package usecase coordinates system first initialization.
package usecase

import (
	"context"

	"github.com/NSObjects/echo-admin/internal/modules/setup/domain"
)

const (
	// DefaultSiteName is used when setup does not provide a site name.
	DefaultSiteName = "Echo Admin"

	minPasswordLength = 8
	maxPasswordLength = 72
)

// StateReader reads persistent installation state.
type StateReader interface {
	Initialized(context.Context) (bool, error)
}

// TransactionRunner executes setup writes in one database transaction.
type TransactionRunner interface {
	RunInitialization(context.Context, func(context.Context, Transaction) error) error
}

// Transaction contains the setup-owned write capabilities required inside the
// first-initialization transaction.
type Transaction interface {
	RequireOpenInstallation(context.Context) error
	InstallRootAuthorization(context.Context) (RootRole, error)
	CreateFirstAdministrator(context.Context, FirstAdministrator) error
	InstallInitialSettings(context.Context, InitialSettings) error
	CompleteInstallation(context.Context) error
}

// Usecase coordinates the one-time first-initialization workflow.
type Usecase struct {
	state  StateReader
	runner TransactionRunner
}

// New creates a setup usecase.
func New(state StateReader, runner TransactionRunner) *Usecase {
	return &Usecase{state: state, runner: runner}
}

// State is the public setup-state DTO.
type State struct {
	Initialized bool `json:"initialized"`
}

// SubmitInput carries the setup form fields.
type SubmitInput struct {
	Username    string
	DisplayName string
	Email       string
	Password    string
	SiteName    string
}

// RootRole is the minimal root-role identity setup needs for first admin creation.
type RootRole struct {
	ID   int64
	Code string
}

// FirstAdministrator is the normalized administrator created during setup.
type FirstAdministrator struct {
	Username     string
	DisplayName  string
	Email        string
	PasswordHash []byte
	RootRoleID   int64
}

// InitialSettings contains the setup-owned initial setting values.
type InitialSettings struct {
	SiteName string
}

func stateFromDomain(state domain.InstallationState) State {
	return State{Initialized: state.Initialized}
}
