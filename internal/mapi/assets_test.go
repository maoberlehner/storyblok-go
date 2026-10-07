package mapi

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestUploadAsset(t *testing.T) {
	var uploaded []byte
	var finished, sentSize, sentToken bool
	mux := http.NewServeMux()
	var api *httptest.Server
	mux.HandleFunc("POST /spaces/1/assets", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		sentSize = body["size"] == "2400x1600" && body["filename"] == "hero.jpg"
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": 9, "post_url": api.URL + "/upload", "pretty_url": "//a.storyblok.com/f/1/2400x1600/abc/hero.jpg",
			"fields": map[string]string{"key": "f/1/2400x1600/abc/hero.jpg", "policy": "p"},
		})
	})
	mux.HandleFunc("POST /upload", func(w http.ResponseWriter, r *http.Request) {
		sentToken = r.Header.Get("Authorization") != ""
		file, _, err := r.FormFile("file")
		if err != nil || r.FormValue("key") != "f/1/2400x1600/abc/hero.jpg" {
			http.Error(w, "bad upload", http.StatusBadRequest)
			return
		}
		uploaded, _ = io.ReadAll(file)
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET /spaces/1/assets/9/finish_upload", func(w http.ResponseWriter, r *http.Request) {
		finished = true
		_, _ = w.Write([]byte(`{}`))
	})
	api = httptest.NewServer(mux)
	defer api.Close()

	client, err := NewClient(api.URL, "1", "token")
	if err != nil {
		t.Fatal(err)
	}
	asset, err := client.UploadAsset(t.Context(), "hero.jpg", []byte("jpeg"), 2400, 1600, "Hero")
	if err != nil {
		t.Fatal(err)
	}
	if asset.ID != 9 || asset.Filename != "https://a.storyblok.com/f/1/2400x1600/abc/hero.jpg" || asset.Alt != "Hero" {
		t.Errorf("asset = %+v", asset)
	}
	if !sentSize || string(uploaded) != "jpeg" || !finished || sentToken {
		t.Errorf("size sent %v, uploaded %q, finished %v, token leaked %v", sentSize, uploaded, finished, sentToken)
	}
}

func TestFindAssetMatchesTheFileName(t *testing.T) {
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("search") != "hero.jpg" {
			t.Errorf("query = %s", r.URL.RawQuery)
		}
		_, _ = w.Write([]byte(`{"assets":[
			{"id":1,"filename":"https://a.storyblok.com/f/1/10x10/x/old-hero.jpg"},
			{"id":2,"filename":"https://a.storyblok.com/f/1/2400x1600/y/hero.jpg","alt":"Hero"}]}`))
	}))
	defer api.Close()
	client, _ := NewClient(api.URL, "1", "token")
	asset, err := client.FindAsset(t.Context(), "hero.jpg")
	if err != nil || asset == nil || asset.ID != 2 || !strings.HasSuffix(asset.Filename, "/hero.jpg") {
		t.Fatalf("asset %+v, err %v", asset, err)
	}
}
