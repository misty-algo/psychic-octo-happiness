package domain

import (
	"testing"
	"time"
)

func TestStatusDictionaryBaselineMatchesInstallationContract(t *testing.T) {
	now := time.Now().UTC()

	dictionary, err := StatusDictionaryBaseline(now)
	if err != nil {
		t.Fatalf("StatusDictionaryBaseline() error = %v, want nil", err)
	}
	if dictionary.Code != StatusDictionaryCode {
		t.Fatalf("Code = %q, want %q", dictionary.Code, StatusDictionaryCode)
	}
	if dictionary.Name != "状态" {
		t.Fatalf("Name = %q, want 状态", dictionary.Name)
	}
	if len(dictionary.Items) != 2 {
		t.Fatalf("Items length = %d, want 2", len(dictionary.Items))
	}

	enabled, disabled := dictionary.Items[0], dictionary.Items[1]
	if enabled.Value != "enabled" || enabled.Label != "启用" {
		t.Fatalf("first item = %q/%q, want 启用/enabled", enabled.Label, enabled.Value)
	}
	if disabled.Value != "disabled" || disabled.Label != "禁用" {
		t.Fatalf("second item = %q/%q, want 禁用/disabled", disabled.Label, disabled.Value)
	}
	if enabled.Sort >= disabled.Sort {
		t.Fatalf("sort order = %d then %d, want ascending", enabled.Sort, disabled.Sort)
	}
}
