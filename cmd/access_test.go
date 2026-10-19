package cmd

import (
	"context"
	"errors"
	"testing"
)

func TestUpgradeAuthorizationCommandUsesConfigAndReturnsUpgradeError(t *testing.T) {
	wantErr := errors.New("upgrade failed")
	var gotConfig string
	command := newUpgradeAuthorizationCommand(func(_ context.Context, configPath string) error {
		gotConfig = configPath
		return wantErr
	})
	command.SetArgs([]string{"--config", "testdata/config.toml"})

	err := command.Execute()
	if !errors.Is(err, wantErr) {
		t.Fatalf("Execute() error = %v, want %v", err, wantErr)
	}
	if gotConfig != "testdata/config.toml" {
		t.Fatalf("config path = %q, want %q", gotConfig, "testdata/config.toml")
	}
}
