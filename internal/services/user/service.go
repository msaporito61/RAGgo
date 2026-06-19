package user

import (
	"database/sql"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"raggo/internal/config"
	"raggo/internal/database"
)

type Service struct {
	DB  *sql.DB
	Cfg *config.Config
}

func (s *Service) SeedAdmin() error {
	_, err := database.GetUserByUsername(s.DB, s.Cfg.AdminUsername)
	if err == nil {
		return nil // already exists
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(s.Cfg.AdminPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return database.CreateUser(s.DB, database.User{
		ID:           uuid.NewString(),
		Username:     s.Cfg.AdminUsername,
		PasswordHash: string(hash),
		Role:         "admin",
	})
}

func (s *Service) Login(username, password string) (access, refresh string, err error) {
	u, err := database.GetUserByUsername(s.DB, username)
	if err != nil {
		return "", "", errors.New("invalid credentials")
	}
	if bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)) != nil {
		return "", "", errors.New("invalid credentials")
	}
	access, err = s.makeToken(u.Username, u.Role, "access", time.Now().Add(30*time.Minute))
	if err != nil {
		return "", "", err
	}
	refresh, err = s.makeToken(u.Username, u.Role, "refresh", time.Now().Add(7*24*time.Hour))
	return
}

func (s *Service) RefreshToken(refreshTok string) (string, error) {
	tok, err := jwt.Parse(refreshTok, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(s.Cfg.SecretKey), nil
	})
	if err != nil || !tok.Valid {
		return "", errors.New("invalid refresh token")
	}
	mc := tok.Claims.(jwt.MapClaims)
	if mc["type"] != "refresh" {
		return "", errors.New("not a refresh token")
	}
	return s.makeToken(mc["sub"].(string), mc["role"].(string), "access", time.Now().Add(30*time.Minute))
}

func (s *Service) makeToken(username, role, typ string, exp time.Time) (string, error) {
	claims := jwt.MapClaims{
		"sub":  username,
		"role": role,
		"type": typ,
		"exp":  exp.Unix(),
		"iat":  time.Now().Unix(),
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(s.Cfg.SecretKey))
}

func (s *Service) CreateUser(username, password, role string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return database.CreateUser(s.DB, database.User{
		ID:           uuid.NewString(),
		Username:     username,
		PasswordHash: string(hash),
		Role:         role,
	})
}

func (s *Service) SetAPIKey(username, plainKey string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(plainKey), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return database.UpdateUserAPIKeyHash(s.DB, username, string(hash))
}

func (s *Service) SetPassword(username, newPassword string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return database.UpdateUserPasswordHash(s.DB, username, string(hash))
}
