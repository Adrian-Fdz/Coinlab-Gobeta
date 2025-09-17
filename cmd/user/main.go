package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"regexp"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"golang.org/x/crypto/bcrypt"
)

type RegisterReq struct {
	Email    string `json:"email"`
	Phone    string `json:"phone"`
	Name     string `json:"name"`
	CURP     string `json:"curp"`
	DOB      string `json:"dob"`      //YYYY-MM-DD
	Age      int    `json:"age"`      //lo guardamos tal cual por ahora
	Password string `json:"password"`
}
type LoginReq struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

var (
	db         *sql.DB
	curpRegexp = regexp.MustCompile(`^[A-Z]{4}\d{6}[HM][A-Z]{5}[0-9A-Z]\d$`)
	emailRx    = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)
	phoneRx    = regexp.MustCompile(`^\+?\d{10,15}$`)
)

func main() {
	dsn := os.Getenv("DB_DSN")
	if dsn == "" {
		log.Fatal("DB_DSN vacío")
	}
	var err error
	db, err = sql.Open("pgx", dsn)
	if err != nil { log.Fatal(err) }
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil { log.Fatal(err) }

	if err := migrate(ctx); err != nil { log.Fatal(err) }

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("ok")) })
	mux.HandleFunc("/register", registerHandler)
	mux.HandleFunc("/login", loginHandler)

	port := ":" + env("PORT", "8081")
	log.Println("user-svc en", port)
	log.Fatal(http.ListenAndServe(port, mux))
}

func migrate(ctx context.Context) error {
	_, err := db.ExecContext(ctx, `
		CREATE EXTENSION IF NOT EXISTS pgcrypto;
		CREATE TABLE IF NOT EXISTS users (
		  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		  email TEXT UNIQUE NOT NULL,
		  phone TEXT UNIQUE NOT NULL,
		  name  TEXT NOT NULL,
		  curp  TEXT UNIQUE NOT NULL,
		  dob   DATE NOT NULL,
		  age   INT  NOT NULL,
		  password_hash TEXT NOT NULL,
		  created_at TIMESTAMP NOT NULL DEFAULT NOW()
		);
	`)
	return err
}

func registerHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost { http.Error(w, "method not allowed", 405); return }
	var req RegisterReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil { http.Error(w, "bad request", 400); return }

	// Validaciones básicas
	if !emailRx.MatchString(req.Email) { http.Error(w, "email inválido", 400); return }
	if !phoneRx.MatchString(req.Phone) { http.Error(w, "teléfono inválido", 400); return }
	if !curpRegexp.MatchString(req.CURP) { http.Error(w, "CURP inválido", 400); return }
	if len(req.Password) < 6 { http.Error(w, "password mínimo 6 chars", 400); return }
	// DOB válida
	dob, err := time.Parse("2006-01-02", req.DOB)
	if err != nil { http.Error(w, "dob inválida, formato YYYY-MM-DD", 400); return }
	if req.Age < 0 || req.Age > 120 { http.Error(w, "edad inválida", 400); return }

	// Hash
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil { http.Error(w, "hash error", 500); return }

	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	_, err = db.ExecContext(ctx, `
	  INSERT INTO users (email, phone, name, curp, dob, age, password_hash)
	  VALUES ($1,$2,$3,$4,$5,$6,$7)
	`, req.Email, req.Phone, req.Name, req.CURP, dob, req.Age, string(hash))
	if err != nil {
		if isUniqueViolation(err) {
			http.Error(w, "email/phone/curp ya registrado", 409); return
		}
		http.Error(w, "db error", 500); return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(201)
	json.NewEncoder(w).Encode(map[string]string{"status": "registered"})
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost { http.Error(w, "method not allowed", 405); return }
	var req LoginReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil { http.Error(w, "bad request", 400); return }
	if !emailRx.MatchString(req.Email) { http.Error(w, "email inválido", 400); return }

	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	var storedHash string
	var name string
	row := db.QueryRowContext(ctx, `SELECT password_hash, name FROM users WHERE email=$1`, req.Email)
	if err := row.Scan(&storedHash, &name); err != nil {
		http.Error(w, "unauthorized", 401); return
	}
	if bcrypt.CompareHashAndPassword([]byte(storedHash), []byte(req.Password)) != nil {
		http.Error(w, "unauthorized", 401); return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "ok",
		"user":   name,
	})
}

func isUniqueViolation(err error) bool {
	// pgx devuelve errores con códigos (23505 unique_violation)
	// fallback simple:
	return err != nil && (contains(err.Error(), "duplicate key value") || contains(err.Error(), "unique constraint"))
}

func contains(s, sub string) bool { return len(s) >= len(sub) && (regexp.MustCompile(regexp.QuoteMeta(sub)).FindStringIndex(s) != nil) }

func env(k, def string) string { if v := os.Getenv(k); v != "" { return v }; return def }

// (extra) si quieres calcular edad del lado del server:
// func calcAge(dob time.Time) int { y, m, d := time.Now().Date(); yy, mm, dd := dob.Date(); age := y - yy; if m < mm || (m==mm && d < dd) { age-- }; return age }