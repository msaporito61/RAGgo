package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"raggo/internal/database"
	"raggo/internal/middleware"
	"raggo/internal/services/collection"
	"raggo/internal/services/document"
)

// privateCIDRs is the list of private/internal CIDR blocks used for SSRF protection.
// Initialized once at package load to avoid rebuilding on every request.
var privateCIDRs []*net.IPNet

func init() {
	blocks := []string{
		"127.0.0.0/8",
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
		"169.254.0.0/16",
		"::1/128",
	}
	for _, cidr := range blocks {
		_, network, err := net.ParseCIDR(cidr)
		if err == nil {
			privateCIDRs = append(privateCIDRs, network)
		}
	}
}

type ScrapeHandler struct {
	IngestSvc *document.IngestService
	CollSvc   *collection.Service
}

func (h *ScrapeHandler) Scrape(w http.ResponseWriter, r *http.Request) {
	c := middleware.ClaimsFromCtx(r.Context())
	var req struct {
		URL        string `json:"url"`
		MaxPages   int    `json:"max_pages"`
		IndexAfter bool   `json:"index"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.URL == "" {
		writeJSON(w, http.StatusBadRequest, errResp("url required"))
		return
	}
	if err := validateURL(req.URL); err != nil {
		writeJSON(w, http.StatusBadRequest, errResp(err.Error()))
		return
	}

	content, err := fetchURL(r.Context(), req.URL)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errResp("failed to fetch URL: "+err.Error()))
		return
	}

	col, err := h.CollSvc.GetOrCreateDefault(r.Context(), c.Username)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errResp("collection error"))
		return
	}

	filename := urlToFilename(req.URL)
	meta, err := h.IngestSvc.Ingest(r.Context(), c.Username, col.ID, col.QdrantName, filename, []byte(content))
	if err != nil {
		writeJSON(w, http.StatusUnprocessableEntity, errResp(err.Error()))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"url":            req.URL,
		"success":        true,
		"indexed_chunks": meta.ChunksCount,
		"message":        "Successfully scraped and indexed",
	})
}

// validateURL blocks private/loopback IP ranges (SSRF protection).
func validateURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("only http/https allowed")
	}
	host := u.Hostname()
	addrs, err := net.LookupHost(host)
	if err != nil {
		return fmt.Errorf("DNS lookup failed")
	}
	for _, addr := range addrs {
		ip := net.ParseIP(addr)
		if ip == nil {
			continue
		}
		for _, network := range privateCIDRs {
			if network.Contains(ip) {
				return fmt.Errorf("URL resolves to private/internal address")
			}
		}
	}
	return nil
}

func fetchURL(ctx context.Context, rawURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "RAGgo/1.0")
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 5<<20)) // 5 MB max
	if err != nil {
		return "", err
	}
	// strip HTML tags naively
	text := stripHTML(string(body))
	return text, nil
}

func stripHTML(s string) string {
	var sb strings.Builder
	inTag := false
	for _, r := range s {
		switch {
		case r == '<':
			inTag = true
		case r == '>':
			inTag = false
			sb.WriteRune(' ')
		case !inTag:
			sb.WriteRune(r)
		}
	}
	return sb.String()
}

func urlToFilename(u string) string {
	parsed, err := url.Parse(u)
	if err != nil {
		return "scraped.txt"
	}
	host := strings.ReplaceAll(parsed.Host, ".", "_")
	path := strings.ReplaceAll(strings.Trim(parsed.Path, "/"), "/", "_")
	name := host
	if path != "" {
		name += "_" + path
	}
	if len(name) > 100 {
		name = name[:100]
	}
	return name + ".txt"
}

// Ensure the interface is satisfied at compile time.
var _ interface {
	Ingest(ctx context.Context, owner string, collectionID int64, qdrantName, filename string, data []byte) (*database.DocumentMeta, error)
} = (*document.IngestService)(nil)
