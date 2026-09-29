package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAdminIPAllowlistControlsAdminEndpoints(t *testing.T) {
	app := newTestApp(t)
	token, admin := registerAndLogin(t, app, "admin-ip@example.test")
	if _, err := app.DB.Exec(`UPDATE users SET role='admin' WHERE id=?`, admin.ID); err != nil {
		t.Fatal(err)
	}

	do := func(method, path, body, ip string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("X-Forwarded-For", ip)
		w := httptest.NewRecorder()
		app.Router().ServeHTTP(w, req)
		return w
	}

	// Empty allowlist remains disabled and does not restrict administrators.
	if w := do(http.MethodGet, "/api/admin/users", "", "198.51.100.10"); w.Code != http.StatusOK {
		t.Fatalf("empty allowlist admin request = %d: %s", w.Code, w.Body.String())
	}

	// Saving a non-empty list must retain the current owner IP, or it is rejected.
	if w := do(http.MethodPut, "/api/admin/ip-allowlist", `{"entries":"203.0.113.0/24"}`, "198.51.100.10"); w.Code != http.StatusBadRequest {
		t.Fatalf("lockout attempt = %d, want 400: %s", w.Code, w.Body.String())
	}

	if w := do(http.MethodPut, "/api/admin/ip-allowlist", `{"entries":"198.51.100.10, 2001:db8::/32"}`, "198.51.100.10"); w.Code != http.StatusOK {
		t.Fatalf("save allowlist = %d: %s", w.Code, w.Body.String())
	}
	if w := do(http.MethodGet, "/api/admin/ip-allowlist", "", "198.51.100.10"); w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "198.51.100.10") {
		t.Fatalf("get allowlist = %d: %s", w.Code, w.Body.String())
	}

	// Every requireAdmin endpoint, including backup configuration, is denied outside the list.
	if w := do(http.MethodGet, "/api/admin/users", "", "203.0.113.10"); w.Code != http.StatusForbidden {
		t.Fatalf("blocked admin users = %d, want 403: %s", w.Code, w.Body.String())
	}
	if w := do(http.MethodGet, "/api/settings/backup-drive", "", "203.0.113.10"); w.Code != http.StatusForbidden {
		t.Fatalf("blocked backup config = %d, want 403: %s", w.Code, w.Body.String())
	}
	if w := do(http.MethodGet, "/api/settings/backup-drive", "", "2001:db8::1"); w.Code != http.StatusOK {
		t.Fatalf("allowed IPv6 backup config = %d: %s", w.Code, w.Body.String())
	}

	// Empty explicitly disables the restriction again.
	if w := do(http.MethodPut, "/api/admin/ip-allowlist", `{"entries":""}`, "198.51.100.10"); w.Code != http.StatusOK {
		t.Fatalf("clear allowlist = %d: %s", w.Code, w.Body.String())
	}
	if w := do(http.MethodGet, "/api/admin/users", "", "203.0.113.10"); w.Code != http.StatusOK {
		t.Fatalf("cleared allowlist admin request = %d: %s", w.Code, w.Body.String())
	}
}

func TestAdminIPAllowlistRejectsInvalidEntries(t *testing.T) {
	app := newTestApp(t)
	token, admin := registerAndLogin(t, app, "admin-ip-invalid@example.test")
	if _, err := app.DB.Exec(`UPDATE users SET role='admin' WHERE id=?`, admin.ID); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPut, "/api/admin/ip-allowlist", strings.NewReader(`{"entries":"198.51.100.10, not-an-ip"}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-Forwarded-For", "198.51.100.10")
	w := httptest.NewRecorder()
	app.Router().ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("invalid allowlist = %d, want 400: %s", w.Code, w.Body.String())
	}
}
