package mcp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
)

// APIClient wraps http.Client with base URL and API key injection.
type APIClient struct {
	BaseURL string
	APIKey  string
	HTTP    *http.Client
}

// NewAPIClient creates an APIClient from environment variables.
func NewAPIClient() *APIClient {
	return &APIClient{
		BaseURL: getenv("RAG_API_URL", "http://localhost:8080"),
		APIKey:  os.Getenv("RAG_API_KEY"),
		HTTP:    &http.Client{},
	}
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// get performs an authenticated GET request and decodes the JSON response.
func (c *APIClient) get(path string) (map[string]any, error) {
	req, err := http.NewRequest(http.MethodGet, c.BaseURL+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-API-Key", c.APIKey)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var result map[string]any
	if err := json.Unmarshal(body, &result); err != nil {
		return map[string]any{"raw": string(body), "status_code": resp.StatusCode}, nil
	}
	return result, nil
}

// post performs an authenticated POST request with a JSON body.
func (c *APIClient) post(path string, body any) (map[string]any, error) {
	b, _ := json.Marshal(body)
	req, err := http.NewRequest(http.MethodPost, c.BaseURL+path, bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-API-Key", c.APIKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	bodyBytes, _ := io.ReadAll(resp.Body)
	var result map[string]any
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return map[string]any{"raw": string(bodyBytes), "status_code": resp.StatusCode}, nil
	}
	return result, nil
}

// put performs an authenticated PUT request with a JSON body.
func (c *APIClient) put(path string, body any) (map[string]any, error) {
	b, _ := json.Marshal(body)
	req, err := http.NewRequest(http.MethodPut, c.BaseURL+path, bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-API-Key", c.APIKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	bodyBytes, _ := io.ReadAll(resp.Body)
	var result map[string]any
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return map[string]any{"raw": string(bodyBytes), "status_code": resp.StatusCode}, nil
	}
	return result, nil
}

// patch performs an authenticated PATCH request with a JSON body.
func (c *APIClient) patch(path string, body any) (map[string]any, error) {
	b, _ := json.Marshal(body)
	req, err := http.NewRequest(http.MethodPatch, c.BaseURL+path, bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-API-Key", c.APIKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	bodyBytes, _ := io.ReadAll(resp.Body)
	var result map[string]any
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return map[string]any{"raw": string(bodyBytes), "status_code": resp.StatusCode}, nil
	}
	return result, nil
}

// delete performs an authenticated DELETE request.
func (c *APIClient) delete(path string) (map[string]any, error) {
	req, err := http.NewRequest(http.MethodDelete, c.BaseURL+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-API-Key", c.APIKey)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	bodyBytes, _ := io.ReadAll(resp.Body)
	if len(bodyBytes) == 0 {
		return map[string]any{"deleted": true, "status_code": resp.StatusCode}, nil
	}
	var result map[string]any
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return map[string]any{"raw": string(bodyBytes), "status_code": resp.StatusCode}, nil
	}
	return result, nil
}

// postNoAuth posts without injecting the API key (for login/refresh).
func (c *APIClient) postNoAuth(path string, body any) (map[string]any, error) {
	b, _ := json.Marshal(body)
	req, err := http.NewRequest(http.MethodPost, c.BaseURL+path, bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	bodyBytes, _ := io.ReadAll(resp.Body)
	var result map[string]any
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return map[string]any{"raw": string(bodyBytes), "status_code": resp.StatusCode}, nil
	}
	return result, nil
}

// --- Tool implementations ---

// HealthCheck calls GET /health.
func (c *APIClient) HealthCheck() (map[string]any, error) {
	return c.get("/health")
}

// ListCollections calls GET /collections.
func (c *APIClient) ListCollections() (map[string]any, error) {
	return c.get("/collections")
}

// CreateCollection calls POST /collections.
func (c *APIClient) CreateCollection(displayName string) (map[string]any, error) {
	return c.post("/collections", map[string]any{"display_name": displayName})
}

// DeleteCollection calls DELETE /collections/{slug}.
func (c *APIClient) DeleteCollection(slug string) (map[string]any, error) {
	return c.delete("/collections/" + slug)
}

// ListDocuments calls GET /documents with pagination.
func (c *APIClient) ListDocuments(page, pageSize int, collectionSlug string) (map[string]any, error) {
	path := fmt.Sprintf("/documents?page=%d&page_size=%d", page, pageSize)
	if collectionSlug != "" {
		path += "&collection_slug=" + collectionSlug
	}
	return c.get(path)
}

// UploadDocument uploads a file via multipart POST /documents/upload.
func (c *APIClient) UploadDocument(filePath, collectionSlug string) (map[string]any, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("open file: %w", err)
	}
	defer f.Close()

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile("file", filepath.Base(filePath))
	if err != nil {
		return nil, err
	}
	if _, err := io.Copy(fw, f); err != nil {
		return nil, err
	}
	if collectionSlug != "" {
		_ = mw.WriteField("collection_slug", collectionSlug)
	}
	mw.Close()

	req, err := http.NewRequest(http.MethodPost, c.BaseURL+"/documents/upload", &buf)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-API-Key", c.APIKey)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	bodyBytes, _ := io.ReadAll(resp.Body)
	var result map[string]any
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return map[string]any{"raw": string(bodyBytes), "status_code": resp.StatusCode}, nil
	}
	return result, nil
}

// DeleteDocument calls DELETE /documents/{id}.
func (c *APIClient) DeleteDocument(id string) (map[string]any, error) {
	return c.delete("/documents/" + id)
}

// MoveDocument calls PATCH /documents/{id}/move.
func (c *APIClient) MoveDocument(id, targetCollectionSlug string) (map[string]any, error) {
	return c.patch("/documents/"+id+"/move", map[string]any{"target_slug": targetCollectionSlug})
}

// ScrapeURL calls POST /documents/scrape.
func (c *APIClient) ScrapeURL(url, collectionSlug string) (map[string]any, error) {
	body := map[string]any{"url": url}
	if collectionSlug != "" {
		body["collection_slug"] = collectionSlug
	}
	return c.post("/documents/scrape", body)
}

// Search calls GET /search.
func (c *APIClient) Search(query, collectionSlug string, limit int) (map[string]any, error) {
	params := url.Values{}
	params.Set("q", query)
	params.Set("limit", fmt.Sprintf("%d", limit))
	if collectionSlug != "" {
		params.Set("collection_slug", collectionSlug)
	}
	return c.get("/search?" + params.Encode())
}

// CreateChatSession calls POST /chat/sessions.
func (c *APIClient) CreateChatSession(name string) (map[string]any, error) {
	body := map[string]any{}
	if name != "" {
		body["name"] = name
	}
	return c.post("/chat/sessions", body)
}

// ListChatSessions calls GET /chat/sessions.
func (c *APIClient) ListChatSessions() (map[string]any, error) {
	return c.get("/chat/sessions")
}

// DeleteChatSession calls DELETE /chat/sessions/{id}.
func (c *APIClient) DeleteChatSession(id string) (map[string]any, error) {
	return c.delete("/chat/sessions/" + id)
}

// Chat calls POST /chat/sessions/{id}/message (non-streaming).
func (c *APIClient) Chat(sessionID, message string, collectionSlugs []string) (map[string]any, error) {
	body := map[string]any{
		"message": message,
	}
	if len(collectionSlugs) > 0 {
		body["collection_slugs"] = collectionSlugs
	}
	return c.post("/chat/sessions/"+sessionID+"/message", body)
}

// Login calls POST /auth/login (no auth required).
func (c *APIClient) Login(username, password string) (map[string]any, error) {
	return c.postNoAuth("/auth/login", map[string]any{
		"username": username,
		"password": password,
	})
}

// RefreshToken calls POST /auth/refresh (no auth required).
func (c *APIClient) RefreshToken(refreshToken string) (map[string]any, error) {
	return c.postNoAuth("/auth/refresh", map[string]any{
		"refresh_token": refreshToken,
	})
}

// ListUsers (admin) calls GET /admin/users.
func (c *APIClient) ListUsers() (map[string]any, error) {
	return c.get("/admin/users")
}

// CreateUser (admin) calls POST /admin/users.
func (c *APIClient) CreateUser(username, password, role string) (map[string]any, error) {
	return c.post("/admin/users", map[string]any{
		"username": username,
		"password": password,
		"role":     role,
	})
}

// DeleteUser (admin) calls DELETE /admin/users/{username}.
func (c *APIClient) DeleteUser(username string) (map[string]any, error) {
	return c.delete("/admin/users/" + username)
}

// SetUserPassword (admin) calls PUT /admin/users/{username}/password.
func (c *APIClient) SetUserPassword(username, password string) (map[string]any, error) {
	return c.put("/admin/users/"+username+"/password", map[string]any{
		"password": password,
	})
}

// SetAPIKey calls POST /users/api-key.
func (c *APIClient) SetAPIKey() (map[string]any, error) {
	return c.post("/users/api-key", map[string]any{})
}

// ListAllCollections (admin) calls GET /admin/collections.
func (c *APIClient) ListAllCollections() (map[string]any, error) {
	return c.get("/admin/collections")
}
