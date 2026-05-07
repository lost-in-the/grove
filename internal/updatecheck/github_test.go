package updatecheck

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestFetchLatest_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, `{"tag_name":"v0.6.0","html_url":"https://example/releases/tag/v0.6.0"}`)
	}))
	defer server.Close()

	rel, err := fetchLatestFromURL(context.Background(), server.URL, 2*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rel.TagName != "v0.6.0" {
		t.Errorf("TagName = %q, want %q", rel.TagName, "v0.6.0")
	}
	if rel.HTMLURL != "https://example/releases/tag/v0.6.0" {
		t.Errorf("HTMLURL = %q", rel.HTMLURL)
	}
}

func TestFetchLatest_404IsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer server.Close()

	_, err := fetchLatestFromURL(context.Background(), server.URL, 2*time.Second)
	if err == nil {
		t.Fatal("expected error on 404")
	}
}

func TestFetchLatest_TimeoutIsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
	}))
	defer server.Close()

	_, err := fetchLatestFromURL(context.Background(), server.URL, 50*time.Millisecond)
	if err == nil {
		t.Fatal("expected timeout error")
	}
}

func TestFetchLatest_MalformedJSONIsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, `{"tag_name": [not a string]}`)
	}))
	defer server.Close()

	_, err := fetchLatestFromURL(context.Background(), server.URL, 2*time.Second)
	if err == nil {
		t.Fatal("expected JSON parse error")
	}
}

func TestFetchLatest_StripsLeadingV(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, `{"tag_name":"v1.2.3","html_url":"https://x"}`)
	}))
	defer server.Close()

	rel, err := fetchLatestFromURL(context.Background(), server.URL, 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if rel.Version() != "1.2.3" {
		t.Errorf("Version() = %q, want %q", rel.Version(), "1.2.3")
	}
}

func TestFetchLatest_EmptyBodyIsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// no body written
	}))
	defer server.Close()

	_, err := fetchLatestFromURL(context.Background(), server.URL, 2*time.Second)
	if err == nil {
		t.Fatal("expected error on empty body")
	}
}
