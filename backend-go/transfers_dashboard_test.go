package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestUploadReservationsDashboardListsActiveReservations(t *testing.T) {
	app := newTestApp(t)
	token, user := registerAndLogin(t, app, "reservations@example.test")
	_, _ = app.DB.Exec(`INSERT INTO connected_accounts (id,user_id,provider,provider_account_id,email,scopes) VALUES (?,?,?,?,?,?)`, "acc", user.ID, "google_drive", "p", "drive@example.test", "[]")
	_, _ = app.DB.Exec(`INSERT INTO upload_reservations (id,user_id,target_connected_account_id,file_name,size_bytes,status,expires_at) VALUES (?,?,?,?,?,?,?)`, "active", user.ID, "acc", "video.mp4", 1234, "reserved", time.Now().Add(time.Hour).UTC().Format(time.RFC3339Nano))
	_, _ = app.DB.Exec(`INSERT INTO upload_reservations (id,user_id,target_connected_account_id,file_name,size_bytes,status) VALUES (?,?,?,?,?,?)`, "done", user.ID, "acc", "done.mp4", 99, "completed")

	req := httptest.NewRequest(http.MethodGet, "/uploads/reservations", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	app.Router().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("reservations = %d: %s", w.Code, w.Body.String())
	}
	var out struct {
		Items []struct {
			ID           string `json:"id"`
			FileName     string `json:"fileName"`
			SizeBytes    string `json:"sizeBytes"`
			AccountEmail string `json:"accountEmail"`
			Status       string `json:"status"`
		} `json:"items"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Items) != 1 || out.Items[0].ID != "active" || out.Items[0].FileName != "video.mp4" || out.Items[0].SizeBytes != "1234" || out.Items[0].AccountEmail != "drive@example.test" || out.Items[0].Status != "reserved" {
		t.Fatalf("items = %+v", out.Items)
	}
}
