package main

import (
	"bytes"
	"encoding/json"
	"golang.org/x/crypto/bcrypt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func registerAndLogin(t *testing.T, app *App, email string) (string, authUser) {
	t.Helper()
	hash, _ := bcrypt.GenerateFromPassword([]byte("password-123"), 10)
	user := authUser{ID: randomID(), Name: "OAuth Test", Email: email}
	app.DB.Exec(`INSERT INTO users (id,name,email,password_hash) VALUES (?,?,?,?)`, user.ID, user.Name, user.Email, hash)

	w := httptest.NewRecorder()
	app.Router().ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewBufferString(`{"email":"`+email+`","password":"password-123"}`)))
	if w.Code != http.StatusOK {
		t.Fatalf("login = %d: %s", w.Code, w.Body.String())
	}
	var session struct {
		AccessToken string   `json:"accessToken"`
		User        authUser `json:"user"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &session); err != nil {
		t.Fatal(err)
	}
	return session.AccessToken, session.User
}

func TestGoogleConfigAndConnectURL(t *testing.T) {
	app := newTestApp(t)
	token, _ := registerAndLogin(t, app, "oauth@example.test")
	r := app.Router()

	config := httptest.NewRequest(http.MethodPost, "/system/google-config", bytes.NewBufferString(`{"clientId":"client-id","clientSecret":"client-secret","redirectUri":"http://localhost:4000/connected-accounts/google/callback"}`))
	config.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, config)
	if w.Code != http.StatusCreated {
		t.Fatalf("config = %d: %s", w.Code, w.Body.String())
	}

	request := httptest.NewRequest(http.MethodGet, "/connected-accounts/google/connect-url", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, request)
	if w.Code != http.StatusOK {
		t.Fatalf("url = %d: %s", w.Code, w.Body.String())
	}
	var response struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(response.URL, "accounts.google.com/o/oauth2") || !strings.Contains(response.URL, "state=") {
		t.Fatalf("unexpected oauth url: %s", response.URL)
	}
}

func TestGoogleConfigReportsAccountAssignmentsAndCap(t *testing.T) {
	app := newTestApp(t)
	token, user := registerAndLogin(t, app, "config-observability@example.test")

	if _, err := app.DB.Exec(`INSERT INTO provider_configs (id,user_id,provider,client_id_encrypted,client_secret_encrypted,redirect_uri,status,label) VALUES (?,?,?,?,?,?,?,?)`, "observed-config", user.ID, "google_drive", app.encrypt("client"), app.encrypt("secret"), "http://localhost:4000/callback", "active", "Observed config"); err != nil {
		t.Fatal(err)
	}
	if _, err := app.DB.Exec(`INSERT INTO connected_accounts (id,user_id,provider_config_id,provider,provider_account_id,email,status) VALUES (?,?,?,?,?,?,?)`, "observed-account", user.ID, "observed-config", "google_drive", "google-account", "drive@example.test", "connected"); err != nil {
		t.Fatal(err)
	}
	if err := app.setSetting("google_api_accounts_per_config", "7"); err != nil {
		t.Fatal(err)
	}

	request := httptest.NewRequest(http.MethodGet, "/system/google-config", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	app.Router().ServeHTTP(w, request)
	if w.Code != http.StatusOK {
		t.Fatalf("configs = %d: %s", w.Code, w.Body.String())
	}

	var response struct {
		Configs []struct {
			ID               string `json:"id"`
			AccountsAssigned int    `json:"accountsAssigned"`
			AccountsCap      int    `json:"accountsCap"`
		} `json:"configs"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if len(response.Configs) != 1 {
		t.Fatalf("configs = %#v", response.Configs)
	}
	if got := response.Configs[0].AccountsAssigned; got != 1 {
		t.Errorf("accountsAssigned = %d, want 1", got)
	}
	if got := response.Configs[0].AccountsCap; got != 7 {
		t.Errorf("accountsCap = %d, want 7", got)
	}
}

func TestGoogleConnectRequiresConfig(t *testing.T) {
	app := newTestApp(t)
	token, _ := registerAndLogin(t, app, "missing-config@example.test")
	request := httptest.NewRequest(http.MethodGet, "/connected-accounts/google/connect-url", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	app.Router().ServeHTTP(w, request)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d: %s", w.Code, w.Body.String())
	}
}
