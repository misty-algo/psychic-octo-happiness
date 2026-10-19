package boot

import (
	"context"
	"strings"
	"testing"

	"github.com/NSObjects/echo-admin/internal/platform/configs"
)

func TestBusinessModulesAreSplitByFoundationCapability(t *testing.T) {
	modules := BusinessModules()
	want := []string{"setup", "access", "identity", "audit", "apitoken", "auth", "settings", "fileasset"}
	if len(modules) != len(want) {
		t.Fatalf("BusinessModules() length = %d, want %d", len(modules), len(want))
	}
	for i := range want {
		if modules[i].Name() != want[i] {
			t.Fatalf("BusinessModules()[%d].Name() = %q, want %q", i, modules[i].Name(), want[i])
		}
	}
}

func TestBusinessModulesRequireMySQLStorage(t *testing.T) {
	app, err := NewApp(configs.Config{}, WithModules(BusinessModules()...))
	if err == nil {
		t.Fatal("NewApp() error = nil, want disabled MySQL storage error")
	}
	if app != nil {
		if closeErr := app.close(context.Background()); closeErr != nil {
			t.Fatalf("close() error = %v", closeErr)
		}
	}
	if !strings.Contains(err.Error(), "mysql") {
		t.Fatalf("NewApp() error = %q, want mysql capability", err)
	}
}
