package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSystemUpdateIsAdminOnlyAndRejectsGitPull(t *testing.T) {
	app := newTestApp(t)
	userToken, _ := registerAndLogin(t, app, "member-update@example.test")

	member := httptest.NewRequest(http.MethodPost, "/system/update", nil)
	member.Header.Set("Authorization", "Bearer "+userToken)
	memberOut := httptest.NewRecorder()
	app.Router().ServeHTTP(memberOut, member)
	if memberOut.Code != http.StatusForbidden {
		t.Fatalf("member update = %d, want 403: %s", memberOut.Code, memberOut.Body.String())
	}

	adminToken, admin := registerAndLogin(t, app, "admin-update@example.test")
	if _, err := app.DB.Exec(`UPDATE users SET role='admin' WHERE id=?`, admin.ID); err != nil {
		t.Fatal(err)
	}
	adminReq := httptest.NewRequest(http.MethodPost, "/system/update", nil)
	adminReq.Header.Set("Authorization", "Bearer "+adminToken)
	adminOut := httptest.NewRecorder()
	app.Router().ServeHTTP(adminOut, adminReq)
	if adminOut.Code != http.StatusConflict {
		t.Fatalf("admin update = %d, want 409: %s", adminOut.Code, adminOut.Body.String())
	}
}
