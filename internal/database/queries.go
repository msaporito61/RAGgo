package database

import (
	"database/sql"
	"time"
)

type User struct {
	ID           string    `json:"id"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"-"`
	APIKeyHash   string    `json:"-"`
	Role         string    `json:"role"`
	CreatedAt    time.Time `json:"created_at"`
}

type Collection struct {
	ID            int64     `json:"id"`
	Slug          string    `json:"slug"`
	DisplayName   string    `json:"display_name"`
	OwnerUsername string    `json:"owner_username"`
	QdrantName    string    `json:"qdrant_name"`
	IsDefault     bool      `json:"is_default"`
	DocumentCount int       `json:"document_count"`
	CreatedAt     time.Time `json:"created_at"`
}

type DocumentMeta struct {
	ID            string
	Filename      string
	FileType      string
	SizeBytes     int64
	ChunksCount   int
	Status        string
	OwnerUsername string
	CollectionID  int64
	UploadedAt    time.Time
}

// Users

func CreateUser(db *sql.DB, u User) error {
	_, err := db.Exec(
		`INSERT INTO users (id, username, password_hash, api_key_hash, role) VALUES (?,?,?,?,?)`,
		u.ID, u.Username, u.PasswordHash, u.APIKeyHash, u.Role,
	)
	return err
}

func GetUserByUsername(db *sql.DB, username string) (*User, error) {
	u := &User{}
	var apiKey sql.NullString
	err := db.QueryRow(
		`SELECT id, username, password_hash, api_key_hash, role, created_at FROM users WHERE username = ?`,
		username,
	).Scan(&u.ID, &u.Username, &u.PasswordHash, &apiKey, &u.Role, &u.CreatedAt)
	if err != nil {
		return nil, err
	}
	u.APIKeyHash = apiKey.String
	return u, nil
}

func GetUserByID(db *sql.DB, id string) (*User, error) {
	u := &User{}
	var apiKey sql.NullString
	err := db.QueryRow(
		`SELECT id, username, password_hash, api_key_hash, role, created_at FROM users WHERE id = ?`,
		id,
	).Scan(&u.ID, &u.Username, &u.PasswordHash, &apiKey, &u.Role, &u.CreatedAt)
	if err != nil {
		return nil, err
	}
	u.APIKeyHash = apiKey.String
	return u, nil
}

func ListUsers(db *sql.DB) ([]User, error) {
	rows, err := db.Query(`SELECT id, username, api_key_hash, role, created_at FROM users ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var users []User
	for rows.Next() {
		var u User
		var apiKey sql.NullString
		if err := rows.Scan(&u.ID, &u.Username, &apiKey, &u.Role, &u.CreatedAt); err != nil {
			return nil, err
		}
		u.APIKeyHash = apiKey.String
		users = append(users, u)
	}
	return users, rows.Err()
}

func UpdateUserPasswordHash(db *sql.DB, username, hash string) error {
	_, err := db.Exec(`UPDATE users SET password_hash = ? WHERE username = ?`, hash, username)
	return err
}

func UpdateUserAPIKeyHash(db *sql.DB, username, hash string) error {
	_, err := db.Exec(`UPDATE users SET api_key_hash = ? WHERE username = ?`, hash, username)
	return err
}

func DeleteUser(db *sql.DB, username string) error {
	_, err := db.Exec(`DELETE FROM users WHERE username = ?`, username)
	return err
}

// Collections

func CreateCollection(db *sql.DB, c Collection) error {
	isDefault := 0
	if c.IsDefault {
		isDefault = 1
	}
	_, err := db.Exec(
		`INSERT INTO collections (slug, display_name, owner_username, qdrant_name, is_default) VALUES (?,?,?,?,?)`,
		c.Slug, c.DisplayName, c.OwnerUsername, c.QdrantName, isDefault,
	)
	return err
}

func GetCollection(db *sql.DB, ownerUsername, slug string) (*Collection, error) {
	c := &Collection{}
	var isDefault int
	err := db.QueryRow(
		`SELECT id, slug, display_name, owner_username, qdrant_name, is_default, document_count, created_at
		 FROM collections WHERE owner_username = ? AND slug = ?`,
		ownerUsername, slug,
	).Scan(&c.ID, &c.Slug, &c.DisplayName, &c.OwnerUsername, &c.QdrantName, &isDefault, &c.DocumentCount, &c.CreatedAt)
	if err != nil {
		return nil, err
	}
	c.IsDefault = isDefault == 1
	return c, nil
}

func GetCollectionByID(db *sql.DB, id int64) (*Collection, error) {
	c := &Collection{}
	var isDefault int
	err := db.QueryRow(
		`SELECT id, slug, display_name, owner_username, qdrant_name, is_default, document_count, created_at
		 FROM collections WHERE id = ?`, id,
	).Scan(&c.ID, &c.Slug, &c.DisplayName, &c.OwnerUsername, &c.QdrantName, &isDefault, &c.DocumentCount, &c.CreatedAt)
	if err != nil {
		return nil, err
	}
	c.IsDefault = isDefault == 1
	return c, nil
}

func ListCollectionsForUser(db *sql.DB, username string) ([]Collection, error) {
	rows, err := db.Query(
		`SELECT id, slug, display_name, owner_username, qdrant_name, is_default, document_count, created_at
		 FROM collections WHERE owner_username = ? ORDER BY is_default DESC, created_at`,
		username,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanCollections(rows)
}

func ListAllCollections(db *sql.DB) ([]Collection, error) {
	rows, err := db.Query(
		`SELECT id, slug, display_name, owner_username, qdrant_name, is_default, document_count, created_at
		 FROM collections ORDER BY owner_username, is_default DESC, created_at`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanCollections(rows)
}

func DeleteCollection(db *sql.DB, id int64) error {
	_, err := db.Exec(`DELETE FROM collections WHERE id = ?`, id)
	return err
}

func IncrementDocumentCount(db *sql.DB, collectionID int64, delta int) error {
	_, err := db.Exec(`UPDATE collections SET document_count = document_count + ? WHERE id = ?`, delta, collectionID)
	return err
}

func scanCollections(rows *sql.Rows) ([]Collection, error) {
	var cols []Collection
	for rows.Next() {
		c := Collection{}
		var isDefault int
		if err := rows.Scan(&c.ID, &c.Slug, &c.DisplayName, &c.OwnerUsername, &c.QdrantName, &isDefault, &c.DocumentCount, &c.CreatedAt); err != nil {
			return nil, err
		}
		c.IsDefault = isDefault == 1
		cols = append(cols, c)
	}
	return cols, rows.Err()
}

// DocumentMeta

func CreateDocumentMeta(db *sql.DB, d DocumentMeta) error {
	_, err := db.Exec(
		`INSERT INTO document_metadata (id, filename, file_type, size_bytes, chunks_count, status, owner_username, collection_id)
		 VALUES (?,?,?,?,?,?,?,?)`,
		d.ID, d.Filename, d.FileType, d.SizeBytes, d.ChunksCount, d.Status, d.OwnerUsername, d.CollectionID,
	)
	return err
}

func GetDocumentMeta(db *sql.DB, id string) (*DocumentMeta, error) {
	d := &DocumentMeta{}
	err := db.QueryRow(
		`SELECT id, filename, file_type, size_bytes, chunks_count, status, owner_username, collection_id, uploaded_at
		 FROM document_metadata WHERE id = ?`, id,
	).Scan(&d.ID, &d.Filename, &d.FileType, &d.SizeBytes, &d.ChunksCount, &d.Status, &d.OwnerUsername, &d.CollectionID, &d.UploadedAt)
	if err != nil {
		return nil, err
	}
	return d, nil
}

func ListDocumentsForUser(db *sql.DB, username string, page, pageSize int) ([]DocumentMeta, int, error) {
	offset := (page - 1) * pageSize
	var total int
	if err := db.QueryRow(`SELECT COUNT(*) FROM document_metadata WHERE owner_username = ?`, username).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := db.Query(
		`SELECT id, filename, file_type, size_bytes, chunks_count, status, owner_username, collection_id, uploaded_at
		 FROM document_metadata WHERE owner_username = ? ORDER BY uploaded_at DESC, rowid DESC LIMIT ? OFFSET ?`,
		username, pageSize, offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var docs []DocumentMeta
	for rows.Next() {
		d := DocumentMeta{}
		if err := rows.Scan(&d.ID, &d.Filename, &d.FileType, &d.SizeBytes, &d.ChunksCount, &d.Status, &d.OwnerUsername, &d.CollectionID, &d.UploadedAt); err != nil {
			return nil, 0, err
		}
		docs = append(docs, d)
	}
	return docs, total, rows.Err()
}

func DeleteDocumentMeta(db *sql.DB, id string) error {
	_, err := db.Exec(`DELETE FROM document_metadata WHERE id = ?`, id)
	return err
}

func UpdateDocumentCollection(db *sql.DB, docID string, newCollectionID int64) error {
	_, err := db.Exec(`UPDATE document_metadata SET collection_id = ? WHERE id = ?`, newCollectionID, docID)
	return err
}

func UpdateDocumentChunksAndStatus(db *sql.DB, id string, chunks int, status string) error {
	_, err := db.Exec(`UPDATE document_metadata SET chunks_count = ?, status = ? WHERE id = ?`, chunks, status, id)
	return err
}

// Audit

func InsertAuditLog(db *sql.DB, username, event, ip string) error {
	_, err := db.Exec(`INSERT INTO audit_log (username, event, ip) VALUES (?,?,?)`, username, event, ip)
	return err
}

// Chat sessions and messages

type ChatSession struct {
	ID        string
	Username  string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type ChatMessage struct {
	ID        int64
	SessionID string
	Role      string
	Content   string
	CreatedAt time.Time
}

func CreateChatSession(db *sql.DB, id, username string) error {
	_, err := db.Exec(`INSERT INTO chat_sessions (id, username) VALUES (?,?)`, id, username)
	return err
}

func GetChatSession(db *sql.DB, id string) (*ChatSession, error) {
	s := &ChatSession{}
	err := db.QueryRow(
		`SELECT id, username, created_at, updated_at FROM chat_sessions WHERE id = ?`, id,
	).Scan(&s.ID, &s.Username, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return s, nil
}

func ListChatSessionsForUser(db *sql.DB, username string) ([]ChatSession, error) {
	rows, err := db.Query(
		`SELECT id, username, created_at, updated_at FROM chat_sessions WHERE username = ? ORDER BY updated_at DESC`,
		username,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var sessions []ChatSession
	for rows.Next() {
		s := ChatSession{}
		if err := rows.Scan(&s.ID, &s.Username, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, err
		}
		sessions = append(sessions, s)
	}
	return sessions, rows.Err()
}

func DeleteChatSession(db *sql.DB, id string) error {
	_, err := db.Exec(`DELETE FROM chat_sessions WHERE id = ?`, id)
	return err
}

func AppendChatMessage(db *sql.DB, sessionID, role, content string) error {
	_, err := db.Exec(`INSERT INTO chat_messages (session_id, role, content) VALUES (?,?,?)`, sessionID, role, content)
	if err != nil {
		return err
	}
	_, err = db.Exec(`UPDATE chat_sessions SET updated_at = CURRENT_TIMESTAMP WHERE id = ?`, sessionID)
	return err
}

func GetSessionMessages(db *sql.DB, sessionID string, limit int) ([]ChatMessage, error) {
	rows, err := db.Query(
		`SELECT id, session_id, role, content, created_at FROM chat_messages
		 WHERE session_id = ? ORDER BY created_at LIMIT ?`, sessionID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var msgs []ChatMessage
	for rows.Next() {
		m := ChatMessage{}
		if err := rows.Scan(&m.ID, &m.SessionID, &m.Role, &m.Content, &m.CreatedAt); err != nil {
			return nil, err
		}
		msgs = append(msgs, m)
	}
	return msgs, rows.Err()
}
