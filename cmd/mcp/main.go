package main

import (
	"context"
	"encoding/json"
	"log"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	mcptools "raggo/internal/mcp"
)

func main() {
	client := mcptools.NewAPIClient()

	s := server.NewMCPServer("raggo", "1.0.0",
		server.WithToolCapabilities(true),
	)

	// ── Tool 1: health_check ─────────────────────────────────────────────────
	s.AddTool(mcp.NewTool("health_check",
		mcp.WithDescription("Check the health status of the RAGgo system"),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		result, err := client.HealthCheck()
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		b, _ := json.Marshal(result)
		return mcp.NewToolResultText(string(b)), nil
	})

	// ── Tool 2: list_collections ─────────────────────────────────────────────
	s.AddTool(mcp.NewTool("list_collections",
		mcp.WithDescription("List all document collections for the authenticated user"),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		result, err := client.ListCollections()
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		b, _ := json.Marshal(result)
		return mcp.NewToolResultText(string(b)), nil
	})

	// ── Tool 3: create_collection ────────────────────────────────────────────
	s.AddTool(mcp.NewTool("create_collection",
		mcp.WithDescription("Create a new document collection"),
		mcp.WithString("display_name", mcp.Required(), mcp.Description("Human-readable collection name")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		name := req.GetString("display_name", "")
		if name == "" {
			return mcp.NewToolResultError("display_name is required"), nil
		}
		result, err := client.CreateCollection(name)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		b, _ := json.Marshal(result)
		return mcp.NewToolResultText(string(b)), nil
	})

	// ── Tool 4: delete_collection ────────────────────────────────────────────
	s.AddTool(mcp.NewTool("delete_collection",
		mcp.WithDescription("Delete a collection by slug (cannot delete the default collection)"),
		mcp.WithString("slug", mcp.Required(), mcp.Description("Collection slug to delete")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		slug := req.GetString("slug", "")
		if slug == "" {
			return mcp.NewToolResultError("slug is required"), nil
		}
		result, err := client.DeleteCollection(slug)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		b, _ := json.Marshal(result)
		return mcp.NewToolResultText(string(b)), nil
	})

	// ── Tool 5: list_documents ───────────────────────────────────────────────
	s.AddTool(mcp.NewTool("list_documents",
		mcp.WithDescription("List indexed documents with pagination"),
		mcp.WithNumber("page", mcp.Description("Page number (default 1)")),
		mcp.WithNumber("page_size", mcp.Description("Items per page (default 20)")),
		mcp.WithString("collection_slug", mcp.Description("Filter by collection slug (empty = all)")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		page := req.GetInt("page", 1)
		pageSize := req.GetInt("page_size", 20)
		collectionSlug := req.GetString("collection_slug", "")
		result, err := client.ListDocuments(page, pageSize, collectionSlug)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		b, _ := json.Marshal(result)
		return mcp.NewToolResultText(string(b)), nil
	})

	// ── Tool 6: upload_document ──────────────────────────────────────────────
	s.AddTool(mcp.NewTool("upload_document",
		mcp.WithDescription("Upload a local file and index it into a collection"),
		mcp.WithString("file_path", mcp.Required(), mcp.Description("Absolute path to the file to upload")),
		mcp.WithString("collection_slug", mcp.Description("Target collection slug (empty = default)")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		filePath := req.GetString("file_path", "")
		if filePath == "" {
			return mcp.NewToolResultError("file_path is required"), nil
		}
		collectionSlug := req.GetString("collection_slug", "")
		result, err := client.UploadDocument(filePath, collectionSlug)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		b, _ := json.Marshal(result)
		return mcp.NewToolResultText(string(b)), nil
	})

	// ── Tool 7: delete_document ──────────────────────────────────────────────
	s.AddTool(mcp.NewTool("delete_document",
		mcp.WithDescription("Delete a document by its ID"),
		mcp.WithString("id", mcp.Required(), mcp.Description("Document ID to delete")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := req.GetString("id", "")
		if id == "" {
			return mcp.NewToolResultError("id is required"), nil
		}
		result, err := client.DeleteDocument(id)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		b, _ := json.Marshal(result)
		return mcp.NewToolResultText(string(b)), nil
	})

	// ── Tool 8: move_document ────────────────────────────────────────────────
	s.AddTool(mcp.NewTool("move_document",
		mcp.WithDescription("Move a document to a different collection"),
		mcp.WithString("id", mcp.Required(), mcp.Description("Document ID to move")),
		mcp.WithString("collection_slug", mcp.Required(), mcp.Description("Target collection slug")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := req.GetString("id", "")
		if id == "" {
			return mcp.NewToolResultError("id is required"), nil
		}
		collectionSlug := req.GetString("collection_slug", "")
		if collectionSlug == "" {
			return mcp.NewToolResultError("collection_slug is required"), nil
		}
		result, err := client.MoveDocument(id, collectionSlug)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		b, _ := json.Marshal(result)
		return mcp.NewToolResultText(string(b)), nil
	})

	// ── Tool 9: scrape_url ───────────────────────────────────────────────────
	s.AddTool(mcp.NewTool("scrape_url",
		mcp.WithDescription("Fetch a URL, extract its text content, and index it"),
		mcp.WithString("url", mcp.Required(), mcp.Description("URL to scrape and index")),
		mcp.WithString("collection_slug", mcp.Description("Target collection slug (empty = default)")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		url := req.GetString("url", "")
		if url == "" {
			return mcp.NewToolResultError("url is required"), nil
		}
		collectionSlug := req.GetString("collection_slug", "")
		result, err := client.ScrapeURL(url, collectionSlug)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		b, _ := json.Marshal(result)
		return mcp.NewToolResultText(string(b)), nil
	})

	// ── Tool 10: search ──────────────────────────────────────────────────────
	s.AddTool(mcp.NewTool("search",
		mcp.WithDescription("Semantic and hybrid search over indexed documents"),
		mcp.WithString("query", mcp.Required(), mcp.Description("Search query text")),
		mcp.WithString("collection_slug", mcp.Description("Collection to search (empty = default)")),
		mcp.WithNumber("limit", mcp.Description("Maximum results to return (default 10)")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		query := req.GetString("query", "")
		if query == "" {
			return mcp.NewToolResultError("query is required"), nil
		}
		collectionSlug := req.GetString("collection_slug", "")
		limit := req.GetInt("limit", 10)
		result, err := client.Search(query, collectionSlug, limit)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		b, _ := json.Marshal(result)
		return mcp.NewToolResultText(string(b)), nil
	})

	// ── Tool 11: create_chat_session ─────────────────────────────────────────
	s.AddTool(mcp.NewTool("create_chat_session",
		mcp.WithDescription("Create a new chat session for multi-turn RAG conversations"),
		mcp.WithString("name", mcp.Description("Optional session name")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		name := req.GetString("name", "")
		result, err := client.CreateChatSession(name)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		b, _ := json.Marshal(result)
		return mcp.NewToolResultText(string(b)), nil
	})

	// ── Tool 12: list_chat_sessions ──────────────────────────────────────────
	s.AddTool(mcp.NewTool("list_chat_sessions",
		mcp.WithDescription("List all chat sessions for the authenticated user"),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		result, err := client.ListChatSessions()
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		b, _ := json.Marshal(result)
		return mcp.NewToolResultText(string(b)), nil
	})

	// ── Tool 13: delete_chat_session ─────────────────────────────────────────
	s.AddTool(mcp.NewTool("delete_chat_session",
		mcp.WithDescription("Delete a chat session and its message history"),
		mcp.WithString("id", mcp.Required(), mcp.Description("Chat session ID")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := req.GetString("id", "")
		if id == "" {
			return mcp.NewToolResultError("id is required"), nil
		}
		result, err := client.DeleteChatSession(id)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		b, _ := json.Marshal(result)
		return mcp.NewToolResultText(string(b)), nil
	})

	// ── Tool 14: chat ────────────────────────────────────────────────────────
	s.AddTool(mcp.NewTool("chat",
		mcp.WithDescription("Send a message to a chat session and receive a RAG-grounded response"),
		mcp.WithString("session_id", mcp.Required(), mcp.Description("Chat session ID")),
		mcp.WithString("message", mcp.Required(), mcp.Description("User message to send")),
		mcp.WithArray("collection_slugs",
			mcp.Description("Collection slugs to include in context (empty = all user collections)"),
			mcp.WithStringItems(),
		),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		sessionID := req.GetString("session_id", "")
		if sessionID == "" {
			return mcp.NewToolResultError("session_id is required"), nil
		}
		message := req.GetString("message", "")
		if message == "" {
			return mcp.NewToolResultError("message is required"), nil
		}
		collectionSlugs := req.GetStringSlice("collection_slugs", nil)
		result, err := client.Chat(sessionID, message, collectionSlugs)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		b, _ := json.Marshal(result)
		return mcp.NewToolResultText(string(b)), nil
	})

	// ── Tool 15: login ───────────────────────────────────────────────────────
	s.AddTool(mcp.NewTool("login",
		mcp.WithDescription("Authenticate with username and password to obtain JWT tokens"),
		mcp.WithString("username", mcp.Required(), mcp.Description("Username")),
		mcp.WithString("password", mcp.Required(), mcp.Description("Password")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		username := req.GetString("username", "")
		if username == "" {
			return mcp.NewToolResultError("username is required"), nil
		}
		password := req.GetString("password", "")
		if password == "" {
			return mcp.NewToolResultError("password is required"), nil
		}
		result, err := client.Login(username, password)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		b, _ := json.Marshal(result)
		return mcp.NewToolResultText(string(b)), nil
	})

	// ── Tool 16: refresh_token ───────────────────────────────────────────────
	s.AddTool(mcp.NewTool("refresh_token",
		mcp.WithDescription("Obtain a new access token using a refresh token"),
		mcp.WithString("refresh_token", mcp.Required(), mcp.Description("Refresh token from a previous login")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		refreshToken := req.GetString("refresh_token", "")
		if refreshToken == "" {
			return mcp.NewToolResultError("refresh_token is required"), nil
		}
		result, err := client.RefreshToken(refreshToken)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		b, _ := json.Marshal(result)
		return mcp.NewToolResultText(string(b)), nil
	})

	// ── Tool 17: list_users (admin) ──────────────────────────────────────────
	s.AddTool(mcp.NewTool("list_users",
		mcp.WithDescription("(Admin) List all users in the system"),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		result, err := client.ListUsers()
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		b, _ := json.Marshal(result)
		return mcp.NewToolResultText(string(b)), nil
	})

	// ── Tool 18: create_user (admin) ─────────────────────────────────────────
	s.AddTool(mcp.NewTool("create_user",
		mcp.WithDescription("(Admin) Create a new user account"),
		mcp.WithString("username", mcp.Required(), mcp.Description("Username for the new user")),
		mcp.WithString("password", mcp.Required(), mcp.Description("Initial password for the new user")),
		mcp.WithString("role", mcp.Description("Role: 'user' or 'admin' (default: user)")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		username := req.GetString("username", "")
		if username == "" {
			return mcp.NewToolResultError("username is required"), nil
		}
		password := req.GetString("password", "")
		if password == "" {
			return mcp.NewToolResultError("password is required"), nil
		}
		role := req.GetString("role", "user")
		result, err := client.CreateUser(username, password, role)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		b, _ := json.Marshal(result)
		return mcp.NewToolResultText(string(b)), nil
	})

	// ── Tool 19: delete_user (admin) ─────────────────────────────────────────
	s.AddTool(mcp.NewTool("delete_user",
		mcp.WithDescription("(Admin) Delete a user account by username"),
		mcp.WithString("username", mcp.Required(), mcp.Description("Username to delete")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		username := req.GetString("username", "")
		if username == "" {
			return mcp.NewToolResultError("username is required"), nil
		}
		result, err := client.DeleteUser(username)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		b, _ := json.Marshal(result)
		return mcp.NewToolResultText(string(b)), nil
	})

	// ── Tool 20: set_user_password (admin) ───────────────────────────────────
	s.AddTool(mcp.NewTool("set_user_password",
		mcp.WithDescription("(Admin) Set a new password for any user"),
		mcp.WithString("username", mcp.Required(), mcp.Description("Username whose password to change")),
		mcp.WithString("password", mcp.Required(), mcp.Description("New password")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		username := req.GetString("username", "")
		if username == "" {
			return mcp.NewToolResultError("username is required"), nil
		}
		password := req.GetString("password", "")
		if password == "" {
			return mcp.NewToolResultError("password is required"), nil
		}
		result, err := client.SetUserPassword(username, password)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		b, _ := json.Marshal(result)
		return mcp.NewToolResultText(string(b)), nil
	})

	// ── Tool 21: set_api_key ─────────────────────────────────────────────────
	s.AddTool(mcp.NewTool("set_api_key",
		mcp.WithDescription("Generate or regenerate the API key for the authenticated user"),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		result, err := client.SetAPIKey()
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		b, _ := json.Marshal(result)
		return mcp.NewToolResultText(string(b)), nil
	})

	// ── Tool 22: list_all_collections (admin) ────────────────────────────────
	s.AddTool(mcp.NewTool("list_all_collections",
		mcp.WithDescription("(Admin) List all collections across all users"),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		result, err := client.ListAllCollections()
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		b, _ := json.Marshal(result)
		return mcp.NewToolResultText(string(b)), nil
	})

	log.Println("RAGgo MCP server starting (stdio transport)")
	if err := server.ServeStdio(s); err != nil {
		log.Fatalf("MCP server error: %v", err)
	}
}
