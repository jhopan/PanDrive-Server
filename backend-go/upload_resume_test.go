package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// Chunk uploads persist their offset in the DB so a server restart can resume.
func TestChunkOffsetsPersisted(t *testing.T) {
	oldBuf, oldMin := splitBufferBytes, splitMinSize
	splitBufferBytes = 0
	splitMinSize = 0
	defer func() { splitBufferBytes = oldBuf; splitMinSize = oldMin }()

	app, token, _ := setupSplitEnv(t)
	_, _ = app.DB.Exec(`UPDATE storage_accounts SET available_bytes=? WHERE connected_account_id='accA'`, int64(1)<<30)
	init := httptest.NewRequest(http.MethodPost, "/uploads/resumable/init", strings.NewReader(`{"fileName":"big.bin","mimeType":"application/octet-stream","sizeBytes":"100"}`))
	init.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	app.Router().ServeHTTP(w, init)
	if w.Code != http.StatusOK && w.Code != http.StatusCreated {
		t.Fatalf("init = %d: %s", w.Code, w.Body.String())
	}
	var session struct {
		SessionID string `json:"sessionId"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &session)
	if session.SessionID == "" {
		t.Fatal("no session id")
	}

	// First chunk: 0-49 returns 308 from the stub -> offset 50 persisted.
	chunk := httptest.NewRequest(http.MethodPut, "/uploads/resumable/chunk/"+session.SessionID, strings.NewReader(strings.Repeat("a", 50)))
	chunk.Header.Set("Authorization", "Bearer "+token)
	chunk.Header.Set("Content-Range", "bytes 0-49/100")
	w2 := httptest.NewRecorder()
	app.Router().ServeHTTP(w2, chunk)
	if w2.Code != http.StatusOK {
		t.Fatalf("chunk = %d: %s", w2.Code, w2.Body.String())
	}
	var uploaded int64
	if err := app.DB.QueryRow(`SELECT uploaded_bytes FROM upload_sessions WHERE id=?`, session.SessionID).Scan(&uploaded); err != nil {
		t.Fatalf("uploaded_bytes missing: %v", err)
	}
	if uploaded != 50 {
		t.Fatalf("uploaded_bytes = %d, want 50", uploaded)
	}
}

// The reconciler fails sessions Google no longer knows about and keeps live ones.
func TestReconcileUploadSessions(t *testing.T) {
	oldBuf, oldMin := splitBufferBytes, splitMinSize
	splitBufferBytes = 0
	splitMinSize = 0
	defer func() { splitBufferBytes = oldBuf; splitMinSize = oldMin }()

	app, _, stub := setupSplitEnv(t)
	var userID string
	_ = app.DB.QueryRow(`SELECT id FROM users WHERE email='split@example.test'`).Scan(&userID)

	// Dead session: served by a tiny server that always 404s.
	deadSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer deadSrv.Close()
	deadURI := app.encrypt(deadSrv.URL + "/upload/session/ghost")
	if _, err := app.DB.Exec(`INSERT INTO upload_sessions (id,user_id,target_connected_account_id,file_name,mime_type,size_bytes,status,google_session_uri,uploaded_bytes) VALUES ('dead',?,'accA','x.bin','application/octet-stream',100,'active',?,50)`, userID, deadURI); err != nil {
		t.Fatal(err)
	}
	// Live session: create via stub init path (register manually through the stub mux).
	// Simpler: point at an existing stub bucket path by pre-registering a session id in the stub.
	stub.mu.Lock()
	stub.sess["/upload/session/live-1"] = "live-1"
	stub.mu.Unlock()
	liveURI := app.encrypt(stub.base + "/upload/session/live-1")
	if _, err := app.DB.Exec(`INSERT INTO upload_sessions (id,user_id,target_connected_account_id,file_name,mime_type,size_bytes,status,google_session_uri,uploaded_bytes) VALUES ('live',?,'accA','y.bin','application/octet-stream',100,'active',?,10)`, userID, liveURI); err != nil {
		t.Fatal(err)
	}

	app.reconcileUploadSessions(testCtx())

	var deadStatus, liveStatus string
	_ = app.DB.QueryRow(`SELECT status FROM upload_sessions WHERE id='dead'`).Scan(&deadStatus)
	_ = app.DB.QueryRow(`SELECT status FROM upload_sessions WHERE id='live'`).Scan(&liveStatus)
	if deadStatus != "failed" {
		t.Fatalf("dead session status = %s, want failed", deadStatus)
	}
	if liveStatus != "active" {
		t.Fatalf("live session status = %s, want active", liveStatus)
	}
	// Live stub answers 308 with Range bytes=0--1 (empty bucket) -> offset stays small.
	var liveOffset int64
	_ = app.DB.QueryRow(`SELECT uploaded_bytes FROM upload_sessions WHERE id='live'`).Scan(&liveOffset)
	if liveOffset < 0 {
		t.Fatalf("negative offset %d", liveOffset)
	}
}

func TestSplitManifestIsDeterministicAndHonestAboutMissingPartChecksums(t *testing.T) {
	manifest, ok := splitManifest("movie.mkv", "video/x-matroska", 9, 2, []splitManifestPart{
		{Index: 2, Size: 4, Checksum: "bbbb"},
		{Index: 1, Size: 5, Checksum: "aaaa"},
	})
	if !ok {
		t.Fatal("manifest should be available when every part has a checksum")
	}
	const want = "3b6c95a0287c1a27ee51bf75a0952732335b211f27fc8bc7102dd6b4eb628e2a" // versioned newline-delimited v1 manifest encoding
	if manifest != want {
		t.Fatalf("manifest = %q, want %q", manifest, want)
	}
	if _, ok := splitManifest("movie.mkv", "video/x-matroska", 9, 2, []splitManifestPart{{Index: 1, Size: 5, Checksum: "aaaa"}, {Index: 2, Size: 4}}); ok {
		t.Fatal("manifest must be unavailable when a legacy part checksum is missing")
	}
}

// Split health reports ok for consistent parts and flags missing/short parts.
func TestSplitHealthEndpoint(t *testing.T) {
	app, token, stub := setupSplitEnv(t)
	var userID string
	_ = app.DB.QueryRow(`SELECT id FROM users WHERE email='split@example.test'`).Scan(&userID)

	// Logical split file with one part recorded in DB.
	if _, err := app.DB.Exec(`INSERT INTO split_files (id,user_id,name,mime_type,size_bytes,part_count,status) VALUES ('sp-h',?,'health.bin','application/octet-stream',10,1,'complete')`, userID); err != nil {
		t.Fatalf("split_files insert: %v", err)
	}
	stub.mu.Lock()
	stub.buckets["drv-health"] = []byte("0123456789")
	stub.mu.Unlock()
	if _, err := app.DB.Exec(`INSERT INTO files (id,user_id,connected_account_id,provider,provider_file_id,name,mime_type,size_bytes,checksum) VALUES ('f-h',?,'accA','google_drive','drv-health','health.part1','application/octet-stream',10,'md5abc')`, userID); err != nil {
		t.Fatalf("files insert: %v", err)
	}
	if _, err := app.DB.Exec(`INSERT INTO split_parts (id,split_id,part_index,file_id,connected_account_id,size_bytes) VALUES ('pp-h','sp-h',1,'f-h','accA',10)`); err != nil {
		t.Fatalf("split_parts insert: %v", err)
	}

	// Healthy case: stub serves size 10 matching DB.
	req := httptest.NewRequest(http.MethodGet, "/files/f-h/split-health", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	app.Router().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("health = %d: %s", w.Code, w.Body.String())
	}
	var out struct {
		Status string `json:"status"`
		Parts  []struct {
			Part  int    `json:"part"`
			Match bool   `json:"match"`
			Error string `json:"error"`
		} `json:"parts"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	if out.Status != "ok" {
		t.Fatalf("health status = %s (parts %+v)", out.Status, out.Parts)
	}

	// Broken case: part shrunk in Drive -> mismatch.
	stub.mu.Lock()
	stub.buckets["drv-health"] = []byte("01234")
	stub.mu.Unlock()
	req2 := httptest.NewRequest(http.MethodGet, "/files/f-h/split-health", nil)
	req2.Header.Set("Authorization", "Bearer "+token)
	w2 := httptest.NewRecorder()
	app.Router().ServeHTTP(w2, req2)
	_ = json.Unmarshal(w2.Body.Bytes(), &out)
	if out.Status != "not_ok" {
		t.Fatalf("health status after shrink = %s, want not_ok", out.Status)
	}
}

// testCtx is a background context with a deadline for reconciler tests.
func testCtx() context.Context {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	go func() { <-time.After(15 * time.Second); cancel() }()
	return ctx
}
