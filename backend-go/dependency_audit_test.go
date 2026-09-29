package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDependencyAuditReportsLockedManifestMetadataWithoutClaimingVulnerabilities(t *testing.T) {
	app := newTestApp(t)
	token, _ := registerAndLogin(t, app, "dependencies@example.test")

	req := httptest.NewRequest(http.MethodGet, "/system/dependencies", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	app.Router().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("dependency report = %d: %s", w.Code, w.Body.String())
	}

	var report struct {
		AuditStatus string `json:"auditStatus"`
		Sources     []struct {
			Name            string `json:"name"`
			LockfilePresent bool   `json:"lockfilePresent"`
			PackageCount    int    `json:"packageCount"`
		}
	}
	if err := json.Unmarshal(w.Body.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if report.AuditStatus != "not_run" {
		t.Fatalf("auditStatus = %q, want not_run", report.AuditStatus)
	}
	if len(report.Sources) != 2 {
		t.Fatalf("source count = %d, want 2", len(report.Sources))
	}
	for _, source := range report.Sources {
		if !source.LockfilePresent || source.PackageCount < 1 {
			t.Fatalf("invalid source metadata: %+v", source)
		}
	}
}
