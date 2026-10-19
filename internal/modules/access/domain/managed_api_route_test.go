package domain

import "testing"

func TestManagedAPIRouteCatalogIsValidAndDefensivelyCopied(t *testing.T) {
	catalog := ManagedAPIRouteCatalog()
	if len(catalog) == 0 {
		t.Fatal("ManagedAPIRouteCatalog() is empty")
	}
	seen := make(map[string]struct{}, len(catalog))
	permissions := make(map[string]struct{})
	for _, permission := range PermissionCatalog() {
		permissions[permission.Token] = struct{}{}
	}
	for _, definition := range catalog {
		key := definition.Method + " " + definition.Pattern
		if definition.Method == "" || definition.Pattern == "" || definition.Description == "" || definition.Group == "" {
			t.Fatalf("ManagedAPIRouteCatalog() definition = %#v, want required fields", definition)
		}
		if _, ok := seen[key]; ok {
			t.Fatalf("ManagedAPIRouteCatalog() duplicate = %s", key)
		}
		seen[key] = struct{}{}
		if definition.Permission == "" {
			continue
		}
		if _, ok := permissions[definition.Permission]; !ok {
			t.Fatalf("ManagedAPIRouteCatalog() %s permission = %q, want catalog permission", key, definition.Permission)
		}
	}

	original := catalog[0]
	catalog[0] = ManagedAPIRouteDefinition{}
	if got := ManagedAPIRouteCatalog()[0]; got != original {
		t.Fatalf("ManagedAPIRouteCatalog()[0] = %#v, want %#v after caller mutation", got, original)
	}
}
