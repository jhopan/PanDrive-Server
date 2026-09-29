package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestListSharesReportsUsageAndSplitDelivery(t *testing.T) {
	app := newTestApp(t)
	token, user := registerAndLogin(t, app, "share-report@example.test")
	if _, err := app.DB.Exec(`INSERT INTO connected_accounts (id,user_id,provider,provider_account_id,email,status) VALUES ('account',?,'google_drive','provider-account','owner@example.test','connected')`, user.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := app.DB.Exec(`INSERT INTO files (id,user_id,connected_account_id,provider,provider_file_id,name,mime_type,size_bytes) VALUES
		('direct-file',?,'account','google_drive','direct-provider','direct.txt','text/plain',10),
		('split-part',?,'account','google_drive','split-provider','part-001','application/octet-stream',40)`, user.ID, user.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := app.DB.Exec(`INSERT INTO split_files (id,user_id,name,mime_type,size_bytes,part_count,status) VALUES ('split',?,'archive.zip','application/zip',100,2,'complete')`, user.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := app.DB.Exec(`INSERT INTO split_parts (id,split_id,part_index,file_id,connected_account_id,size_bytes) VALUES ('part','split',1,'split-part','account',40)`); err != nil {
		t.Fatal(err)
	}
	if _, err := app.DB.Exec(`INSERT INTO share_links (id,user_id,file_id,connected_account_id,provider_file_id,url,max_downloads,download_count,split_id) VALUES
		('direct-share',?,'direct-file','account','direct-provider','https://example.test/s/direct-share',7,3,NULL),
		('split-share',?,'split-part','account','split-provider','https://example.test/s/split-share',5,2,'split')`, user.ID, user.ID); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/shares", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	app.Router().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("list shares = %d: %s", w.Code, w.Body.String())
	}
	var response struct {
		Shares []struct {
			ID             string `json:"id"`
			Name           string `json:"name"`
			SizeBytes      string `json:"sizeBytes"`
			DownloadCount  int64  `json:"downloadCount"`
			MaxDownloads   *int64 `json:"maxDownloads"`
			ShareMode      string `json:"shareMode"`
			SplitName      string `json:"splitName"`
			SplitSizeBytes string `json:"splitSizeBytes"`
		} `json:"shares"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if len(response.Shares) != 2 {
		t.Fatalf("share count = %d, want 2", len(response.Shares))
	}
	byID := map[string]struct {
		ID             string `json:"id"`
		Name           string `json:"name"`
		SizeBytes      string `json:"sizeBytes"`
		DownloadCount  int64  `json:"downloadCount"`
		MaxDownloads   *int64 `json:"maxDownloads"`
		ShareMode      string `json:"shareMode"`
		SplitName      string `json:"splitName"`
		SplitSizeBytes string `json:"splitSizeBytes"`
	}{}
	for _, share := range response.Shares {
		byID[share.ID] = share
	}
	if share := byID["direct-share"]; share.DownloadCount != 3 || share.MaxDownloads == nil || *share.MaxDownloads != 7 || share.ShareMode != "direct_google" || share.Name != "direct.txt" || share.SizeBytes != "10" || share.SplitName != "" || share.SplitSizeBytes != "" {
		t.Fatalf("direct share report = %+v", share)
	}
	if share := byID["split-share"]; share.DownloadCount != 2 || share.MaxDownloads == nil || *share.MaxDownloads != 5 || share.ShareMode != "vps_merge" || share.Name != "archive.zip" || share.SizeBytes != "100" || share.SplitName != "archive.zip" || share.SplitSizeBytes != "100" {
		t.Fatalf("split share report = %+v", share)
	}
}
