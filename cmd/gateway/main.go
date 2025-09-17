package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"sync/atomic"
)

var counter uint64

// consulta Consul para obtener instancias saludables
func getUserSvcInstances() ([]string, error) {
	resp, err := http.Get("http://consul:8500/v1/health/service/user-svc?passing=true")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var arr []struct {
		Service struct {
			Address string
			Port    int
		}
	}
	if err := json.NewDecoder(resp.Body).Decode(&arr); err != nil {
		return nil, err
	}

	var out []string
	for _, it := range arr {
		out = append(out, fmt.Sprintf("http://%s:%d", it.Service.Address, it.Service.Port))
	}
	return out, nil
}

func loginProxy(w http.ResponseWriter, r *http.Request) {
	instances, err := getUserSvcInstances()
	if err != nil || len(instances) == 0 {
		http.Error(w, "no upstream available", http.StatusBadGateway)
		return
	}

	// round robin
	idx := atomic.AddUint64(&counter, 1)
	target := instances[idx%uint64(len(instances))]

	u, _ := url.Parse(target + "/login")
	req, _ := http.NewRequest(http.MethodPost, u.String(), r.Body)
	req.Header = r.Header

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		http.Error(w, "upstream error", 502)
		return
	}
	defer resp.Body.Close()

	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

func registerProxy(w http.ResponseWriter, r *http.Request) {
	instances, err := getUserSvcInstances()
	if err != nil || len(instances) == 0 {
		http.Error(w, "no upstream available", 502)
		return
	}

	// round robin
	idx := atomic.AddUint64(&counter, 1)
	target := instances[idx%uint64(len(instances))]

	u, _ := url.Parse(target + "/register")
	req, _ := http.NewRequest(r.Method, u.String(), r.Body) // ewwnvio del metodo post
	req.Header = r.Header

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		http.Error(w, "upstream error", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

func keysProxy(w http.ResponseWriter, r *http.Request) {
	// Consul -> keys
	resp, err := http.Get("http://consul:8500/v1/health/service/keys-svc?passing=true")
	if err != nil {
		http.Error(w, "no upstream available", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	var arr []struct {
		Service struct {
			Address string
			Port    int
		}
	}
	if err := json.NewDecoder(resp.Body).Decode(&arr); err != nil || len(arr) == 0 {
		http.Error(w, "no upstream available", http.StatusBadGateway)
		return
	}

	// Round robin
	idx := atomic.AddUint64(&counter, 1)
	target := fmt.Sprintf("http://%s:%d", arr[idx%uint64(len(arr))].Service.Address, arr[idx%uint64(len(arr))].Service.Port)

	// Construir la request original proxy a keys-svc
	u, _ := url.Parse(target + "/keys")
	req, _ := http.NewRequest(r.Method, u.String(), r.Body)
	req.Header = r.Header

	upResp, err := http.DefaultClient.Do(req)
	if err != nil {
		http.Error(w, "upstream error", http.StatusBadGateway)
		return
	}
	defer upResp.Body.Close()

	w.WriteHeader(upResp.StatusCode)
	io.Copy(w, upResp.Body)
}


func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/login", loginProxy)
	mux.HandleFunc("/register", registerProxy)
	mux.HandleFunc("/keys", keysProxy)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("ok")) })


	port := ":8080" //Default 8080
	if p := os.Getenv("PORT"); p != "" {
		port = ":" + p
	}

	log.Println("gateway en", port)
	log.Fatal(http.ListenAndServe(port, mux))
}
