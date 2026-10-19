package domain

import (
	"errors"
	"testing"
	"time"
)

func TestRestoreFileObjectAllowsExternalHTTPURLWithUnknownSize(t *testing.T) {
	file, err := RestoreFileObject(1, "manual.pdf", "https://cdn.example.com/manual.pdf?version=1", 0, "external/url", 0, zeroTime())
	if err != nil {
		t.Fatalf("RestoreFileObject(external url) error = %v", err)
	}
	if file.URL != "https://cdn.example.com/manual.pdf?version=1" {
		t.Fatalf("URL = %q, want external url", file.URL)
	}
}

func TestRestoreFileObjectRejectsUnsafeURL(t *testing.T) {
	_, err := RestoreFileObject(1, "bad.txt", "javascript:alert(1)", 0, "external/url", 0, zeroTime())
	if !errors.Is(err, ErrInvalidFileURL) {
		t.Fatalf("RestoreFileObject(javascript url) error = %v, want %v", err, ErrInvalidFileURL)
	}
}

func TestRestoreFileCategoryRejectsSelfParent(t *testing.T) {
	_, err := RestoreFileCategory(7, 7, "合同", zeroTime(), zeroTime())
	if !errors.Is(err, ErrInvalidCategoryID) {
		t.Fatalf("RestoreFileCategory(self parent) error = %v, want %v", err, ErrInvalidCategoryID)
	}
}

func zeroTime() time.Time {
	return time.Time{}
}
