package main

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

type Key struct {
	ID           string    `json:"id"`
	Provider     string    `json:"provider"`
	APIKeyLast4  string    `json:"api_key_last4"`
	CreatedAt    time.Time `json:"created_at"`
}

type CreateKeyReq struct {
	Provider string `json:"provider"`
	APIKey   string `json:"api_key"`
	Secret   string `json:"secret"`
}

var db *sql.DB
var masterKey []byte

func main() {
	dsn := os.Getenv("DB_DSN")
	if dsn == "" {
		log.Fatal("DB_DSN vacío")
	}
	var err error
	db, err = sql.Open("pgx", dsn)
	if err != nil {
		log.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		log.Fatal(err)
	}

	// leer master key
	mk := os.Getenv("KEYS_MASTER")
	if mk == "" {
		log.Fatal("KEYS_MASTER vacío")
	}
	if strings.HasPrefix(mk, "base64:") {
		masterKey, err = base64.StdEncoding.DecodeString(strings.TrimPrefix(mk, "base64:"))
		if err != nil {
			log.Fatal("KEYS_MASTER decode error:", err)
		}
	} else {
		masterKey = []byte(mk)
	}
	if len(masterKey) != 32 {
		log.Fatal("KEYS_MASTER debe ser 32 bytes")
	}

	if err := migrate(ctx); err != nil {
		log.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("ok")) })
	mux.HandleFunc("/keys", keysHandler)

	port := ":" + env("PORT", "8083")
	log.Println("keys-svc en", port)
	log.Fatal(http.ListenAndServe(port, mux))
}

func migrate(ctx context.Context) error {
	_, err := db.ExecContext(ctx, `
		CREATE EXTENSION IF NOT EXISTS pgcrypto;
		CREATE TABLE IF NOT EXISTS api_keys (
		  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		  provider TEXT NOT NULL,
		  api_key_hash BYTEA NOT NULL,
		  api_key_last4 TEXT NOT NULL,
		  secret_ct BYTEA NOT NULL,
		  secret_nonce BYTEA NOT NULL,
		  created_at TIMESTAMP NOT NULL DEFAULT NOW()
		);
	`)
	return err
}

func keysHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		var req CreateKeyReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad request", 400)
			return
		}
		if req.APIKey == "" || req.Secret == "" || req.Provider == "" {
			http.Error(w, "faltan campos", 400)
			return
		}

		apiKeyHash := sha256.Sum256([]byte(req.APIKey))
		last4 := ""
		if len(req.APIKey) >= 4 {
			last4 = req.APIKey[len(req.APIKey)-4:]
		} else {
			last4 = req.APIKey
		}

		ct, nonce, err := encrypt(req.Secret, masterKey)
		if err != nil {
			http.Error(w, "encrypt error", 500)
			return
		}

		var id string
		err = db.QueryRow(
			`INSERT INTO api_keys(provider, api_key_hash, api_key_last4, secret_ct, secret_nonce)
			 VALUES($1,$2,$3,$4,$5) RETURNING id`,
			req.Provider, apiKeyHash[:], last4, ct, nonce).Scan(&id)
		if err != nil {
			http.Error(w, "db error", 500)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"id":            id,
			"provider":      req.Provider,
			"api_key_last4": last4,
		})

	case http.MethodGet:
		rows, err := db.Query(`SELECT id, provider, api_key_last4, created_at FROM api_keys ORDER BY created_at DESC LIMIT 50`)
		if err != nil {
			http.Error(w, "db error", 500)
			return
		}
		defer rows.Close()

		var list []Key
		for rows.Next() {
			var k Key
			if err := rows.Scan(&k.ID, &k.Provider, &k.APIKeyLast4, &k.CreatedAt); err == nil {
				list = append(list, k)
			}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(list)

	default:
		http.Error(w, "method not allowed", 405)
	}
}

func encrypt(plaintext string, key []byte) ([]byte, []byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, nil, err
	}
	ciphertext := gcm.Seal(nil, nonce, []byte(plaintext), nil)
	return ciphertext, nonce, nil
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
