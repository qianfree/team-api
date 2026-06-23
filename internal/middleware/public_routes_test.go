package middleware

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

type publicRoutePattern struct {
	path    string
	dynamic bool
}

func TestPublicRouteWhitelistsMatchMetaTags(t *testing.T) {
	adminRoutes := scanPublicMetaRoutes(t, filepath.Join("..", "..", "api", "admin", "v1"), "/api/admin")
	assertExactPublicRoutes(t, "admin", adminRoutes, adminPublicPaths)

	tenantRoutes := scanPublicMetaRoutes(t, filepath.Join("..", "..", "api", "tenant", "v1"), "/api/tenant")
	for _, route := range tenantRoutes {
		if !tenantPublicRouteAllowed(route.path) {
			t.Fatalf("tenant public route %q is tagged middleware:\"-\" but is missing from tenant auth whitelist", route.path)
		}
	}
	assertNoStaleTenantPublicWhitelist(t, tenantRoutes)
}

func scanPublicMetaRoutes(t *testing.T, dir string, basePath string) []publicRoutePattern {
	t.Helper()

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read api dir %s: %v", dir, err)
	}

	metaPattern := regexp.MustCompile("`([^`]+)`")
	pathPattern := regexp.MustCompile(`path:"([^"]+)"`)
	routes := make([]publicRoutePattern, 0)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
			continue
		}
		content, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			t.Fatalf("read api file %s: %v", entry.Name(), err)
		}
		for _, match := range metaPattern.FindAllStringSubmatch(string(content), -1) {
			meta := match[1]
			if !strings.Contains(meta, `middleware:"-"`) {
				continue
			}
			pathMatch := pathPattern.FindStringSubmatch(meta)
			if len(pathMatch) != 2 {
				t.Fatalf("public meta in %s missing path: %s", entry.Name(), meta)
			}
			path := basePath + pathMatch[1]
			routes = append(routes, publicRoutePattern{
				path:    path,
				dynamic: strings.Contains(path, "{"),
			})
		}
	}
	return routes
}

func assertExactPublicRoutes(t *testing.T, name string, routes []publicRoutePattern, whitelist map[string]bool) {
	t.Helper()

	found := make(map[string]bool, len(routes))
	for _, route := range routes {
		if route.dynamic {
			t.Fatalf("%s public route %q is dynamic; add prefix/suffix whitelist support before exposing it", name, route.path)
		}
		if !whitelist[route.path] {
			t.Fatalf("%s public route %q is tagged middleware:\"-\" but is missing from whitelist", name, route.path)
		}
		found[route.path] = true
	}

	for path := range whitelist {
		if !found[path] {
			t.Fatalf("%s whitelist contains stale path %q without matching middleware:\"-\" API tag", name, path)
		}
	}
}

func tenantPublicRouteAllowed(path string) bool {
	if tenantPublicPaths[path] {
		return true
	}
	for _, prefix := range tenantPublicPrefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return strings.HasPrefix(path, "/api/tenant/oauth/") && strings.HasSuffix(path, "/callback")
}

func assertNoStaleTenantPublicWhitelist(t *testing.T, routes []publicRoutePattern) {
	t.Helper()

	foundExact := make(map[string]bool, len(routes))
	for _, route := range routes {
		if !route.dynamic {
			foundExact[route.path] = true
		}
	}
	for path := range tenantPublicPaths {
		if !foundExact[path] {
			t.Fatalf("tenant whitelist contains stale exact path %q without matching middleware:\"-\" API tag", path)
		}
	}

	for _, prefix := range tenantPublicPrefixes {
		matched := false
		for _, route := range routes {
			if strings.HasPrefix(route.path, prefix) {
				matched = true
				break
			}
		}
		if !matched {
			t.Fatalf("tenant whitelist contains stale prefix %q without matching middleware:\"-\" API tag", prefix)
		}
	}
}
