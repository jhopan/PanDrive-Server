package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSplitDownloadLimitsConcurrentStreamsPerUser(t *testing.T) {
	app, token := newSplitRangeEnv(t, []string{"DATA"})
	app.Config.MaxConcurrentSplitDownloads = 1

	started := make(chan struct{})
	release := make(chan struct{})
	var calls int
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			close(started)
			<-release
		}
		_, _ = w.Write([]byte("DATA"))
	}))
	t.Cleanup(upstream.Close)
	app.GoogleDriveAPIURL = upstream.URL

	download := func() *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/files/part-file-1/download", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		app.Router().ServeHTTP(w, req)
		return w
	}

	firstDone := make(chan *httptest.ResponseRecorder, 1)
	go func() { firstDone <- download() }()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("first split download did not reach upstream")
	}

	limited := download()
	if limited.Code != http.StatusTooManyRequests {
		t.Fatalf("limited status = %d: %s", limited.Code, limited.Body.String())
	}
	if limited.Header().Get("Retry-After") == "" {
		t.Fatal("limited response missing Retry-After")
	}

	close(release)
	if first := <-firstDone; first.Code != http.StatusOK {
		t.Fatalf("first status = %d: %s", first.Code, first.Body.String())
	}
	if next := download(); next.Code != http.StatusOK {
		t.Fatalf("released status = %d: %s", next.Code, next.Body.String())
	}
}
