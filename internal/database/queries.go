package database

import (
	"database/sql"
	"time"
)

type User struct {
	ID           string
	Username     string
	PasswordHash string
	APIKeyHash   string
	Role         string
	CreatedAt    time.Time
}

type Collection struct {
	ID            int64
	Slug          string
	DisplayName   string
	OwnerUsername string
	QdrantName    string
	IsDefault     bool
	DocumentCount int
	CreatedAt     time.Time
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
	rows, err := db.Query(`SELECT id, username, role, created_at FROM users ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var users []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Username, &u.Role, &u.CreatedAt); err != nil {
			return nil, err
		}
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
