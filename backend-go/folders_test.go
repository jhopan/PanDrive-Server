package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCreateAndListVirtualFolders(t *testing.T) {
	app := newTestApp(t)
	token, _ := registerAndLogin(t, app, "folders@example.test")
	r := app.Router()

	request := httptest.NewRequest(http.MethodPost, "/folders", bytes.NewBufferString(`{"name":"Dokumen"}`))
	request.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, request)
	if w.Code != http.StatusCreated {
		t.Fatalf("create = %d: %s", w.Code, w.Body.String())
	}
	var created struct {
		Folder struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"folder"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.Folder.Name != "Dokumen" || created.Folder.ID == "" {
		t.Fatalf("folder = %#v", created.Folder)
	}

	request = httptest.NewRequest(http.MethodGet, "/folders", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, request)
	if w.Code != http.StatusOK {
		t.Fatalf("list = %d: %s", w.Code, w.Body.String())
	}
	var listed struct {
		Folders []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"folders"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &listed); err != nil {
		t.Fatal(err)
	}
	if len(listed.Folders) != 1 || listed.Folders[0].ID != created.Folder.ID {
		t.Fatalf("folders = %#v", listed.Folders)
	}
}

func TestFolderUploadPlanCreatesNestedFoldersAndReservesCapacity(t *testing.T) {
	app := newTestApp(t)
	token, user := registerAndLogin(t, app, "folder-plan@example.test")
	for _, account := range []struct {
		id, providerID string
		available      int64
	}{{"account-a", "drive-a", 100}, {"account-b", "drive-b", 100}} {
		if _, err := app.DB.Exec(`INSERT INTO connected_accounts (id,user_id,provider,provider_account_id,email,scopes) VALUES (?,?,?,?,?,?)`, account.id, user.ID, "google_drive", account.providerID, account.id+"@example.test", "[]"); err != nil {
			t.Fatal(err)
		}
		if _, err := app.DB.Exec(`INSERT INTO storage_accounts (id,connected_account_id,total_bytes,used_bytes,available_bytes) VALUES (?,?,?,?,?)`, "storage-"+account.id, account.id, account.available, 0, account.available); err != nil {
			t.Fatal(err)
		}
	}

	req := httptest.NewRequest(http.MethodPost, "/uploads/folder-plan", newJSONBody(t, map[string]any{"files": []map[string]string{
		{"relativePath": "Photos/2026/a.jpg", "sizeBytes": "80"},
		{"relativePath": "Photos/2026/b.jpg", "sizeBytes": "80"},
	}}))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	app.Router().ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d: %s", w.Code, w.Body.String())
	}
	var body struct {
		PlanID string `json:"planId"`
		Files  []struct {
			RelativePath    string `json:"relativePath"`
			FolderID        string `json:"folderId"`
			TargetAccountID string `json:"targetAccountId"`
		} `json:"files"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.PlanID == "" || len(body.Files) != 2 || body.Files[0].FolderID == "" || body.Files[0].FolderID != body.Files[1].FolderID || body.Files[0].TargetAccountID == body.Files[1].TargetAccountID {
		t.Fatalf("plan = %#v", body)
	}
	var folderCount int
	if err := app.DB.QueryRow(`SELECT COUNT(*) FROM folders WHERE user_id=?`, user.ID).Scan(&folderCount); err != nil || folderCount != 2 {
		t.Fatalf("folders = %d, err = %v", folderCount, err)
	}
	var available int64
	if err := app.DB.QueryRow(`SELECT COALESCE(SUM(available_bytes),0) FROM storage_accounts`).Scan(&available); err != nil || available != 40 {
		t.Fatalf("available = %d, err = %v", available, err)
	}
}

func TestFoldersAreUserScoped(t *testing.T) {
	app := newTestApp(t)
	tokenA, _ := registerAndLogin(t, app, "folder-a@example.test")
	tokenB, _ := registerAndLogin(t, app, "folder-b@example.test")
	r := app.Router()
	request := httptest.NewRequest(http.MethodPost, "/folders", bytes.NewBufferString(`{"name":"Private"}`))
	request.Header.Set("Authorization", "Bearer "+tokenA)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, request)
	request = httptest.NewRequest(http.MethodGet, "/folders", nil)
	request.Header.Set("Authorization", "Bearer "+tokenB)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, request)
	var listed struct {
		Folders []any `json:"folders"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &listed)
	if len(listed.Folders) != 0 {
		t.Fatalf("other user sees %#v", listed.Folders)
	}
}
