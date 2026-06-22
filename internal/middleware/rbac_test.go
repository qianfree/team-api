package middleware

import "testing"

func TestAdminPublicPathsSkipPermissionMapping(t *testing.T) {
	for path := range adminPublicPaths {
		if !isAdminPublicPath(path) {
			t.Fatalf("admin public path %q is not recognized by shared public path helper", path)
		}
		if perm := matchPermission("POST", path); perm != "" {
			t.Fatalf("admin public path %q unexpectedly maps to permission %q", path, perm)
		}
	}
}

func TestMatchPermissionForProtectedAdminRoute(t *testing.T) {
	if perm := matchPermission("GET", "/api/admin/users"); perm != "user:view" {
		t.Fatalf("matchPermission(GET /api/admin/users) = %q, want user:view", perm)
	}
}
