package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestIncompleteSplitRecoveryDiscardAndDetach(t *testing.T) {
	app, token, stub := setupSplitEnv(t)
	var userID string
	if err := app.DB.QueryRow(`SELECT id FROM users WHERE email='split@example.test'`).Scan(&userID); err != nil {
		t.Fatal(err)
	}
	if _, err := app.DB.Exec(`INSERT INTO split_files (id,user_id,name,mime_type,size_bytes,part_count,status) VALUES ('discard-split',?,'movie.mkv','video/x-matroska',12,2,'incomplete'),('keep-split',?,'clip.mkv','video/x-matroska',8,1,'incomplete')`, userID, userID); err != nil {
		t.Fatal(err)
	}
	if _, err := app.DB.Exec(`INSERT INTO files (id,user_id,connected_account_id,provider,provider_file_id,name,mime_type,size_bytes) VALUES ('discard-file-1',?,'accA','google_drive','discard-drive-1','pd-split-discard.part001','application/octet-stream',6),('discard-file-2',?,'accB','google_drive','discard-drive-2','pd-split-discard.part002','application/octet-stream',6),('keep-file',?,'accA','google_drive','keep-drive','pd-split-keep.part001','application/octet-stream',8)`, userID, userID, userID); err != nil {
		t.Fatal(err)
	}
	if _, err := app.DB.Exec(`INSERT INTO split_parts (id,split_id,part_index,file_id,connected_account_id,size_bytes) VALUES ('discard-part-1','discard-split',1,'discard-file-1','accA',6),('discard-part-2','discard-split',2,'discard-file-2','accB',6),('keep-part','keep-split',1,'keep-file','accA',8)`); err != nil {
		t.Fatal(err)
	}

	r := app.Router()
	discard := httptest.NewRequest(http.MethodPost, "/uploads/split/discard-split/discard", nil)
	discard.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, discard)
	if w.Code != http.StatusOK {
		t.Fatalf("discard = %d: %s", w.Code, w.Body.String())
	}
	if len(stub.deleted) != 2 || !stub.deleted["discard-drive-1"] || !stub.deleted["discard-drive-2"] {
		t.Fatalf("Drive deletes = %#v, want both discard parts", stub.deleted)
	}
	for _, query := range []string{
		`SELECT COUNT(*) FROM split_files WHERE id='discard-split'`,
		`SELECT COUNT(*) FROM split_parts WHERE split_id='discard-split'`,
		`SELECT COUNT(*) FROM files WHERE id IN ('discard-file-1','discard-file-2')`,
	} {
		var count int
		if err := app.DB.QueryRow(query).Scan(&count); err != nil || count != 0 {
			t.Fatalf("after discard %q = %d, err=%v", query, count, err)
		}
	}

	detach := httptest.NewRequest(http.MethodPost, "/uploads/split/keep-split/detach", nil)
	detach.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, detach)
	if w.Code != http.StatusOK {
		t.Fatalf("detach = %d: %s", w.Code, w.Body.String())
	}
	var name, status string
	if err := app.DB.QueryRow(`SELECT name,status FROM files WHERE id='keep-file'`).Scan(&name, &status); err != nil {
		t.Fatal(err)
	}
	if name != "clip.mkv.part001" || status != "active" {
		t.Fatalf("detached file = name=%q status=%q", name, status)
	}
	var count int
	if err := app.DB.QueryRow(`SELECT COUNT(*) FROM split_files WHERE id='keep-split'`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("keep split rows = %d, err=%v", count, err)
	}
}

func TestIncompleteSplitRecoveryRejectsCompleteAndOtherUsers(t *testing.T) {
	app, token, _ := setupSplitEnv(t)
	var userID string
	if err := app.DB.QueryRow(`SELECT id FROM users WHERE email='split@example.test'`).Scan(&userID); err != nil {
		t.Fatal(err)
	}
	if _, err := app.DB.Exec(`INSERT INTO split_files (id,user_id,name,mime_type,size_bytes,part_count,status) VALUES ('complete-split',?,'done.bin','application/octet-stream',1,1,'complete')`, userID); err != nil {
		t.Fatal(err)
	}
	r := app.Router()
	for _, path := range []string{"/uploads/split/complete-split/discard", "/uploads/split/complete-split/detach", "/uploads/split/missing/discard"} {
		req := httptest.NewRequest(http.MethodPost, path, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusNotFound {
			t.Fatalf("%s = %d: %s", path, w.Code, w.Body.String())
		}
	}
}
