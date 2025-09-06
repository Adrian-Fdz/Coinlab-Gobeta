package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
)

// estructura para recibir login
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", 400)
		return
	}

	// simulamos validación
	if req.Username == "admin" && req.Password == "1234" {
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	} else {
		http.Error(w, "unauthorized", 401)
	}
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("ok"))
}

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/login", loginHandler)
	mux.HandleFunc("/healthz", healthHandler)

	port := ":8081"
	if p := os.Getenv("PORT"); p != "" {
		port = ":" + p
	}

	log.Println("user-svc en", port)
	log.Fatal(http.ListenAndServe(port, mux))
}
