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
		http.Error(w, "no upstream available", 502)
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

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/login", loginProxy)

	port := ":8080"
	if p := os.Getenv("PORT"); p != "" {
		port = ":" + p
	}

	log.Println("gateway en", port)
	log.Fatal(http.ListenAndServe(port, mux))
}
