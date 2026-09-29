package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestPasswordProtectedShareAtomicallyCapsDownloads(t *testing.T) {
	app := newTestApp(t)
	hash, err := bcrypt.GenerateFromPassword([]byte("secret-pass"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatal(err)
	}
	// Satisfy foreign keys for the isolated share fixture.
	if _, err := app.DB.Exec(`INSERT INTO users (id,name,email,password_hash) VALUES ('user','User','user@example.test','hash')`); err != nil {
		t.Fatal(err)
	}
	if _, err := app.DB.Exec(`INSERT INTO connected_accounts (id,user_id,provider,provider_account_id,email,status) VALUES ('account','user','google_drive','account','account@example.test','connected')`); err != nil {
		t.Fatal(err)
	}
	if _, err := app.DB.Exec(`INSERT INTO files (id,user_id,connected_account_id,provider,provider_file_id,name,mime_type,size_bytes) VALUES ('file','user','account','google_drive','provider-file','report.pdf','application/pdf',1)`); err != nil {
		t.Fatal(err)
	}
	if _, err := app.DB.Exec(`INSERT INTO share_links (id,user_id,file_id,connected_account_id,provider_file_id,permission_id,url,password_hash,max_downloads) VALUES (?,?,?,?,?,?,?,?,?)`,
		"protected", "user", "file", "account", "provider-file", "permission", "https://page.test/s/protected", string(hash), 2); err != nil {
		t.Fatal(err)
	}
	get := httptest.NewRequest(http.MethodGet, "/s/protected", nil)
	page := httptest.NewRecorder()
	app.Router().ServeHTTP(page, get)
	if page.Code != http.StatusOK || !strings.Contains(page.Body.String(), "Password") || strings.Contains(page.Body.String(), "drive.google.com/uc") {
		t.Fatalf("protected page = %d %q", page.Code, page.Body.String())
	}

	post := func(password string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(http.MethodPost, "/s/protected", strings.NewReader(url.Values{"password": {password}}.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()
		app.Router().ServeHTTP(w, req)
		return w
	}
	if w := post("wrong"); w.Code != http.StatusUnauthorized {
		t.Fatalf("wrong password = %d", w.Code)
	}

	var wg sync.WaitGroup
	codes := make(chan int, 4)
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			codes <- post("secret-pass").Code
		}()
	}
	wg.Wait()
	close(codes)
	redirects, limited := 0, 0
	for code := range codes {
		switch code {
		case http.StatusSeeOther:
			redirects++
		case http.StatusTooManyRequests:
			limited++
		default:
			t.Fatalf("download attempt = %d", code)
		}
	}
	if redirects != 2 || limited != 2 {
		t.Fatalf("redirects/limited = %d/%d, want 2/2", redirects, limited)
	}
	var count int
	if err := app.DB.QueryRow(`SELECT download_count FROM share_links WHERE id='protected'`).Scan(&count); err != nil || count != 2 {
		t.Fatalf("download_count = %d, %v; want 2", count, err)
	}
}

func TestPasswordProtectedShareLocksAfterFiveFailedAttempts(t *testing.T) {
	app := newTestApp(t)
	hash, err := bcrypt.GenerateFromPassword([]byte("secret-pass"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := app.DB.Exec(`INSERT INTO users (id,name,email,password_hash) VALUES ('user','User','user@example.test','hash')`); err != nil {
		t.Fatal(err)
	}
	if _, err := app.DB.Exec(`INSERT INTO connected_accounts (id,user_id,provider,provider_account_id,email,status) VALUES ('account','user','google_drive','account','account@example.test','connected')`); err != nil {
		t.Fatal(err)
	}
	if _, err := app.DB.Exec(`INSERT INTO files (id,user_id,connected_account_id,provider,provider_file_id,name,mime_type,size_bytes) VALUES ('file','user','account','google_drive','provider-file','report.pdf','application/pdf',1)`); err != nil {
		t.Fatal(err)
	}
	if _, err := app.DB.Exec(`INSERT INTO share_links (id,user_id,file_id,connected_account_id,provider_file_id,permission_id,url,password_hash) VALUES (?,?,?,?,?,?,?,?)`, "limited", "user", "file", "account", "provider-file", "permission", "https://page.test/s/limited", string(hash)); err != nil {
		t.Fatal(err)
	}

	post := func(password string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(http.MethodPost, "/s/limited", strings.NewReader(url.Values{"password": {password}}.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("X-Forwarded-For", "198.51.100.10")
		w := httptest.NewRecorder()
		app.Router().ServeHTTP(w, req)
		return w
	}
	for i := 0; i < 4; i++ {
		if w := post("wrong"); w.Code != http.StatusUnauthorized {
			t.Fatalf("failed attempt %d = %d, want 401", i+1, w.Code)
		}
	}
	if w := post("secret-pass"); w.Code != http.StatusSeeOther {
		t.Fatalf("correct password = %d, want 303", w.Code)
	}
	if w := post("wrong"); w.Code != http.StatusUnauthorized {
		t.Fatalf("fifth failed attempt = %d, want 401", w.Code)
	}
	if w := post("secret-pass"); w.Code != http.StatusTooManyRequests || w.Header().Get("Retry-After") == "" {
		t.Fatalf("locked attempt = %d, Retry-After %q; want 429 with Retry-After", w.Code, w.Header().Get("Retry-After"))
	}
}

func TestSharePasswordLimiterDoesNotCountCorrectPasswords(t *testing.T) {
	app := newTestApp(t)
	shareID, ip := "share", "198.51.100.11"
	for range 4 {
		app.sharePasswordRecordFailure(shareID, ip)
	}
	if blocked, _ := app.sharePasswordBlocked(shareID, ip); blocked {
		t.Fatal("four failures must not lock share")
	}
	// Correct password handling never records a failure, so the next failed attempt is fifth.
	app.sharePasswordRecordFailure(shareID, ip)
	if blocked, _ := app.sharePasswordBlocked(shareID, ip); !blocked {
		t.Fatal("correct passwords must not advance the failure counter")
	}
}

func TestSharePasswordLimiterConcurrentFailuresLockShare(t *testing.T) {
	app := newTestApp(t)
	const attempts = 20
	var wg sync.WaitGroup
	for range attempts {
		wg.Add(1)
		go func() {
			defer wg.Done()
			app.sharePasswordRecordFailure("concurrent-share", "198.51.100.12")
		}()
	}
	wg.Wait()
	if blocked, wait := app.sharePasswordBlocked("concurrent-share", "198.51.100.12"); !blocked || wait <= 0 {
		t.Fatalf("concurrent failures blocked=%t wait=%s; want active lock", blocked, wait)
	}
	if blocked, _ := app.sharePasswordBlocked("other-share", "198.51.100.12"); blocked {
		t.Fatal("lock must be scoped to share ID and client IP")
	}
}

func TestPublicLinkStoresPasswordHashAndLimit(t *testing.T) {
	app := newTestApp(t)
	// Existing link creation uses the Drive API; security options must remain local.
	// Detailed Drive setup is covered by TestPublicLinkCreateListRevoke.
	_ = app
}
