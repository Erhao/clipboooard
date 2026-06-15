package main

import (
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
)

func main() {
	store, err := NewStore("clipboooard.db")
	if err != nil {
		log.Fatal("store init:", err)
	}
	defer store.Close()

	hub := NewHub()
	go hub.Run()

	h := &Handler{store: store, hub: hub}

	if err := os.MkdirAll("uploads", 0755); err != nil {
		log.Fatal(err)
	}

	initAuth(store)

	mux := http.NewServeMux()

	mux.HandleFunc("POST /api/login", h.Login)
	mux.HandleFunc("POST /api/clip", authMiddleware(h.CreateClip))
	mux.HandleFunc("GET /api/clip/latest", authMiddleware(h.GetLatestClip))
	mux.HandleFunc("GET /api/clip/{id}", authMiddleware(h.GetClip))
	mux.HandleFunc("GET /api/clips", authMiddleware(h.ListClips))
	mux.HandleFunc("POST /api/upload", authMiddleware(h.UploadFile))
	mux.HandleFunc("GET /api/file/{id}", authMiddleware(h.DownloadFile))
	mux.HandleFunc("GET /ws", h.HandleWebSocket)

	frontendDir := filepath.Join("..", "frontend", "dist")
	if _, err := os.Stat(frontendDir); err == nil {
		fs := http.FileServer(http.Dir(frontendDir))
		mux.Handle("/", fs)
		log.Println("serving frontend from", frontendDir)
	} else {
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/" {
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte(`{"service":"clipboooard","status":"ok"}`))
				return
			}
			http.NotFound(w, r)
		})
	}

	addr := "0.0.0.0:8090"

	lanIP := getLanIP()

	log.Println("clipboooard started:")
	log.Println("  本机:     http://localhost:8090")
	if lanIP != "" {
		log.Println("  局域网:   http://" + lanIP + ":8090")
	}
	log.Fatal(http.ListenAndServe(addr, withCORS(mux)))
}

func getLanIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ""
	}
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() && ipnet.IP.To4() != nil {
			return ipnet.IP.String()
		}
	}
	return ""
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}
