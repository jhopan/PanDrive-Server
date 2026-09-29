package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestStreamServesPartialVideoInline(t *testing.T) {
	app := newTestApp(t)
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("alt") != "media" || r.Header.Get("Range") != "bytes=2-4" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Range", "bytes 2-4/6")
		w.Header().Set("Content-Length", "3")
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write([]byte("cde"))
	}))
	t.Cleanup(api.Close)
	app.GoogleDriveAPIURL = api.URL + "/drive/v3"

	token, user := registerAndLogin(t, app, "stream@example.test")
	_, _ = app.DB.Exec(`INSERT INTO provider_configs (id,user_id,provider,client_id_encrypted,client_secret_encrypted,redirect_uri) VALUES (?,?,?,?,?,?)`, "config", user.ID, "google_drive", app.encrypt("client"), app.encrypt("secret"), "http://localhost:4000/callback")
	_, err := app.DB.Exec(`INSERT INTO connected_accounts (id,user_id,provider,provider_account_id,email,access_token_encrypted,refresh_token_encrypted,token_expires_at,scopes,provider_config_id) VALUES (?,?,?,?,?,?,?,?,?,?)`, "account", user.ID, "google_drive", "google", "drive@example.test", app.encrypt("access"), app.encrypt("refresh"), "2099-01-01T00:00:00Z", "[]", "config")
	if err != nil {
		t.Fatal(err)
	}
	_, err = app.DB.Exec(`INSERT INTO files (id,user_id,connected_account_id,provider,provider_file_id,name,mime_type,size_bytes) VALUES (?,?,?,?,?,?,?,?)`, "file", user.ID, "account", "google_drive", "google-file", "movie.mp4", "video/mp4", 6)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/files/file/stream", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Range", "bytes=2-4")
	w := httptest.NewRecorder()
	app.Router().ServeHTTP(w, req)

	if w.Code != http.StatusPartialContent {
		t.Fatalf("status = %d: %s", w.Code, w.Body.String())
	}
	if got := w.Header().Get("Content-Disposition"); got != `inline; filename="movie.mp4"` {
		t.Fatalf("disposition = %q", got)
	}
	if got := w.Header().Get("Content-Range"); got != "bytes 2-4/6" {
		t.Fatalf("content-range = %q", got)
	}
}
