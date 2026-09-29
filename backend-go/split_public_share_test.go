package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// A public link created from any physical split part must point to the logical
// split, and the public download endpoint must stream the reconstructed file.
func TestPublicShareStreamsLogicalSplit(t *testing.T) {
	app, token, stub := setupSplitEnv(t)
	var userID string
	if err := app.DB.QueryRow(`SELECT id FROM users WHERE email='split@example.test'`).Scan(&userID); err != nil {
		t.Fatal(err)
	}
	// The Drive permission request made by publicPermission.
	stub.mu.Lock()
	stub.buckets["part-a"] = []byte("hello ")
	stub.buckets["part-b"] = []byte("world")
	stub.mu.Unlock()
	_, _ = app.DB.Exec(`UPDATE storage_accounts SET available_bytes=? WHERE connected_account_id='accA'`, int64(1)<<30)
	_, _ = app.DB.Exec(`UPDATE storage_accounts SET available_bytes=? WHERE connected_account_id='accB'`, int64(1)<<30)
	_, _ = app.DB.Exec(`INSERT INTO split_files (id,user_id,name,mime_type,size_bytes,part_count,status) VALUES ('logical',?,'joined.txt','text/plain',11,2,'complete')`, userID)
	for _, row := range []struct{ id, provider, account string; size int64 }{{"a", "part-a", "accA", 6}, {"b", "part-b", "accB", 5}} {
		_, err := app.DB.Exec(`INSERT INTO files (id,user_id,connected_account_id,provider,provider_file_id,name,mime_type,size_bytes) VALUES (?,?,?,?,?,?,?,?)`, row.id, userID, row.account, "google_drive", row.provider, row.provider, "text/plain", row.size)
		if err != nil { t.Fatal(err) }
	}
	_, _ = app.DB.Exec(`INSERT INTO split_parts (id,split_id,part_index,file_id,connected_account_id,size_bytes) VALUES ('pa','logical',1,'a','accA',6),('pb','logical',2,'b','accB',5)`)

	create := httptest.NewRequest(http.MethodPost, "/files/a/public-link", strings.NewReader(`{"maxDownloads":1}`))
	create.Header.Set("Authorization", "Bearer "+token)
	create.Header.Set("Content-Type", "application/json")
	created := httptest.NewRecorder()
	app.Router().ServeHTTP(created, create)
	if created.Code != http.StatusOK { t.Fatalf("create = %d: %s", created.Code, created.Body.String()) }
	shareID := strings.TrimPrefix(strings.TrimSpace(created.Body.String()), "")
	_ = shareID
	var id string
	if err := app.DB.QueryRow(`SELECT id FROM share_links WHERE user_id=? ORDER BY created_at DESC LIMIT 1`, userID).Scan(&id); err != nil { t.Fatal(err) }
	var linked string
	if err := app.DB.QueryRow(`SELECT file_id FROM share_links WHERE id=?`, id).Scan(&linked); err != nil { t.Fatal(err) }
	if linked != "a" { t.Fatalf("share file_id = %q, want clicked part a", linked) }

	// Logical marker must exist so the public route knows to stream the split.
	var splitID string
	if err := app.DB.QueryRow(`SELECT split_id FROM share_links WHERE id=?`, id).Scan(&splitID); err != nil { t.Fatal(err) }
	if splitID != "logical" { t.Fatalf("share split_id = %q", splitID) }

	download := httptest.NewRequest(http.MethodPost, "/s/"+id, strings.NewReader(""))
	download.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	out := httptest.NewRecorder()
	app.Router().ServeHTTP(out, download)
	if out.Code != http.StatusOK { t.Fatalf("download = %d: %s", out.Code, out.Body.String()) }
	if out.Body.String() != "hello world" { t.Fatalf("body = %q", out.Body.String()) }
}
