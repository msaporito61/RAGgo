package mcp

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthCheck(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "healthy"})
	}))
	defer srv.Close()

	client := &APIClient{BaseURL: srv.URL, APIKey: "test", HTTP: &http.Client{}}
	result, err := client.get("/health")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if result["status"] != "healthy" {
		t.Errorf("unexpected status: %v", result["status"])
	}
}

func TestListCollections(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-API-Key") != "testkey" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []any{}})
	}))
	defer srv.Close()

	client := &APIClient{BaseURL: srv.URL, APIKey: "testkey", HTTP: &http.Client{}}
	result, err := client.ListCollections()
	if err != nil {
		t.Fatalf("ListCollections: %v", err)
	}
	if _, ok := result["data"]; !ok {
		t.Errorf("expected 'data' key in result, got: %v", result)
	}
}

func TestCreateCollection(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{"slug": "my-collection", "display_name": "My Collection"})
	}))
	defer srv.Close()

	client := &APIClient{BaseURL: srv.URL, APIKey: "test", HTTP: &http.Client{}}
	result, err := client.CreateCollection("My Collection")
	if err != nil {
		t.Fatalf("CreateCollection: %v", err)
	}
	if result["slug"] != "my-collection" {
		t.Errorf("unexpected slug: %v", result["slug"])
	}
	if gotBody["display_name"] != "My Collection" {
		t.Errorf("unexpected body display_name: %v", gotBody["display_name"])
	}
}

func TestSearch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("q")
		if q == "" {
			http.Error(w, "query required", http.StatusBadRequest)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []any{}, "query": q, "total": 0})
	}))
	defer srv.Close()

	client := &APIClient{BaseURL: srv.URL, APIKey: "test", HTTP: &http.Client{}}
	result, err := client.Search("test query", "", 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if result["query"] != "test query" {
		t.Errorf("unexpected query: %v", result["query"])
	}
}

func TestLogin(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["username"] == "admin" && body["password"] == "secret" {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"access_token":  "tok",
				"refresh_token": "ref",
			})
		} else {
			http.Error(w, "invalid credentials", http.StatusUnauthorized)
		}
	}))
	defer srv.Close()

	client := &APIClient{BaseURL: srv.URL, APIKey: "", HTTP: &http.Client{}}
	result, err := client.Login("admin", "secret")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if result["access_token"] != "tok" {
		t.Errorf("unexpected access_token: %v", result["access_token"])
	}
}

func TestDeleteDocument(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			http.Error(w, "wrong method", http.StatusMethodNotAllowed)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	client := &APIClient{BaseURL: srv.URL, APIKey: "test", HTTP: &http.Client{}}
	result, err := client.DeleteDocument("doc-123")
	if err != nil {
		t.Fatalf("DeleteDocument: %v", err)
	}
	_ = result // may be empty map for 204
}
