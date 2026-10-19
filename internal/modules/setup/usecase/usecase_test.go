package usecase_test

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"golang.org/x/crypto/bcrypt"

	"github.com/NSObjects/echo-admin/internal/modules/setup/usecase"
	"github.com/NSObjects/echo-admin/internal/platform/apperr"
)

func TestStateReturnsInstallationState(t *testing.T) {
	uc := usecase.New(&stateReader{initialized: true}, &runnerSpy{})

	state, err := uc.State(context.Background())
	if err != nil {
		t.Fatalf("State() error = %v", err)
	}
	if !state.Initialized {
		t.Fatal("State().Initialized = false, want true")
	}
}

func TestSubmitCreatesFoundationInOneTransaction(t *testing.T) {
	runner := &runnerSpy{tx: &transactionSpy{rootRole: usecase.RootRole{ID: 7, Code: "super_admin"}}}
	uc := usecase.New(&stateReader{}, runner)

	state, err := uc.Submit(context.Background(), usecase.SubmitInput{
		Username:    "  RootAdmin  ",
		DisplayName: "  Root Admin  ",
		Email:       "  ROOT@example.com  ",
		Password:    "secret-password",
		SiteName:    "",
	})
	if err != nil {
		t.Fatalf("Submit() error = %v", err)
	}
	if !state.Initialized {
		t.Fatal("Submit().Initialized = false, want true")
	}

	wantOrder := []string{
		"RequireOpenInstallation",
		"InstallRootAuthorization",
		"CreateFirstAdministrator",
		"InstallInitialSettings",
		"CompleteInstallation",
	}
	if !reflect.DeepEqual(runner.tx.calls, wantOrder) {
		t.Fatalf("transaction calls = %v, want %v", runner.tx.calls, wantOrder)
	}
	gotAdmin := runner.tx.admin
	if gotAdmin.Username != "rootadmin" {
		t.Fatalf("created admin username = %q, want rootadmin", gotAdmin.Username)
	}
	if gotAdmin.DisplayName != "Root Admin" {
		t.Fatalf("created admin display name = %q, want Root Admin", gotAdmin.DisplayName)
	}
	if gotAdmin.Email != "ROOT@example.com" {
		t.Fatalf("created admin email = %q, want ROOT@example.com", gotAdmin.Email)
	}
	if gotAdmin.RootRoleID != 7 {
		t.Fatalf("created admin root role id = %d, want 7", gotAdmin.RootRoleID)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(gotAdmin.PasswordHash), []byte("secret-password")); err != nil {
		t.Fatalf("created admin password hash mismatch: %v", err)
	}
	if runner.tx.settings.SiteName != usecase.DefaultSiteName {
		t.Fatalf("initial site name = %q, want %q", runner.tx.settings.SiteName, usecase.DefaultSiteName)
	}
}

func TestSubmitRejectsInvalidInputBeforeTransaction(t *testing.T) {
	tests := []struct {
		name  string
		input usecase.SubmitInput
	}{
		{
			name: "empty username",
			input: usecase.SubmitInput{
				Username:    "",
				DisplayName: "Root Admin",
				Password:    "secret-password",
			},
		},
		{
			name: "short password",
			input: usecase.SubmitInput{
				Username:    "root",
				DisplayName: "Root Admin",
				Password:    "short",
			},
		},
		{
			name: "invalid email",
			input: usecase.SubmitInput{
				Username:    "root",
				DisplayName: "Root Admin",
				Email:       "not-an-email",
				Password:    "secret-password",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &runnerSpy{tx: &transactionSpy{rootRole: usecase.RootRole{ID: 7, Code: "super_admin"}}}
			uc := usecase.New(&stateReader{}, runner)

			_, err := uc.Submit(context.Background(), tt.input)
			if code := appCode(t, err); code != apperr.ErrBadRequest {
				t.Fatalf("Submit() code = %d, want %d", code, apperr.ErrBadRequest)
			}
			if runner.calls != 0 {
				t.Fatalf("transaction calls = %d, want 0", runner.calls)
			}
		})
	}
}

func TestSubmitReturnsConflictWhenInstallationAlreadyCompleted(t *testing.T) {
	runner := &runnerSpy{
		tx: &transactionSpy{
			rootRole:   usecase.RootRole{ID: 7, Code: "super_admin"},
			requireErr: apperr.NewConflict("system is already initialized"),
		},
	}
	uc := usecase.New(&stateReader{}, runner)

	_, err := uc.Submit(context.Background(), validInput())
	if code := appCode(t, err); code != apperr.ErrConflict {
		t.Fatalf("Submit() code = %d, want %d", code, apperr.ErrConflict)
	}
	if runner.tx.completed {
		t.Fatal("CompleteInstallation() was called after open-installation conflict")
	}
}

func TestSubmitRejectsBrokenRootAuthorizationBridge(t *testing.T) {
	runner := &runnerSpy{tx: &transactionSpy{rootRole: usecase.RootRole{ID: 0, Code: "super_admin"}}}
	uc := usecase.New(&stateReader{}, runner)

	_, err := uc.Submit(context.Background(), validInput())
	if code := appCode(t, err); code != apperr.ErrInternalServer {
		t.Fatalf("Submit() code = %d, want %d", code, apperr.ErrInternalServer)
	}
	if runner.tx.completed {
		t.Fatal("CompleteInstallation() was called with invalid root role")
	}
}

func TestSubmitPropagatesTransactionFailureWithoutCompleting(t *testing.T) {
	runner := &runnerSpy{
		tx: &transactionSpy{
			rootRole:  usecase.RootRole{ID: 7, Code: "super_admin"},
			createErr: errors.New("identity store failed"),
		},
	}
	uc := usecase.New(&stateReader{}, runner)

	_, err := uc.Submit(context.Background(), validInput())
	if err == nil {
		t.Fatal("Submit() error = nil, want transaction failure")
	}
	if runner.tx.completed {
		t.Fatal("CompleteInstallation() was called after admin creation failure")
	}
}

func validInput() usecase.SubmitInput {
	return usecase.SubmitInput{
		Username:    "root",
		DisplayName: "Root Admin",
		Email:       "root@example.com",
		Password:    "secret-password",
		SiteName:    "Echo Admin",
	}
}

func appCode(t *testing.T, err error) int {
	t.Helper()
	if err == nil {
		t.Fatal("error = nil, want application error")
	}
	appErr, ok := apperr.Parse(err)
	if !ok {
		t.Fatalf("error = %T %[1]v, want application error", err)
	}
	return appErr.Code()
}

type stateReader struct {
	initialized bool
	err         error
}

func (s *stateReader) Initialized(context.Context) (bool, error) {
	return s.initialized, s.err
}

type runnerSpy struct {
	tx    *transactionSpy
	calls int
}

func (r *runnerSpy) RunInitialization(ctx context.Context, fn func(context.Context, usecase.Transaction) error) error {
	r.calls++
	return fn(ctx, r.tx)
}

type transactionSpy struct {
	rootRole    usecase.RootRole
	requireErr  error
	authErr     error
	createErr   error
	settingsErr error
	completeErr error

	calls     []string
	admin     usecase.FirstAdministrator
	settings  usecase.InitialSettings
	completed bool
}

func (t *transactionSpy) RequireOpenInstallation(context.Context) error {
	t.calls = append(t.calls, "RequireOpenInstallation")
	return t.requireErr
}

func (t *transactionSpy) InstallRootAuthorization(context.Context) (usecase.RootRole, error) {
	t.calls = append(t.calls, "InstallRootAuthorization")
	return t.rootRole, t.authErr
}

func (t *transactionSpy) CreateFirstAdministrator(_ context.Context, input usecase.FirstAdministrator) error {
	t.calls = append(t.calls, "CreateFirstAdministrator")
	t.admin = input
	return t.createErr
}

func (t *transactionSpy) InstallInitialSettings(_ context.Context, input usecase.InitialSettings) error {
	t.calls = append(t.calls, "InstallInitialSettings")
	t.settings = input
	return t.settingsErr
}

func (t *transactionSpy) CompleteInstallation(context.Context) error {
	t.calls = append(t.calls, "CompleteInstallation")
	t.completed = true
	return t.completeErr
}
