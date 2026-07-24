package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golf/internal/api"
	"golf/internal/database"
)

func main() {
	log.SetFlags(log.Ltime)
	address := flag.String("addr", envOr("GOLF_ADDR", ":8080"), "HTTP listen address")
	databasePath := flag.String("db", envOr("GOLF_DB", "golf.db"), "SQLite database path")
	staticDir := flag.String("static", envOr("GOLF_STATIC", "web/dist"), "built frontend directory")
	flag.Parse()

	store, err := database.Open(*databasePath)
	if err != nil {
		log.Fatal(err)
	}
	defer store.Close()

	apiHandler := api.New(store)
	handler := spaHandler(apiHandler, *staticDir)
	server := &http.Server{
		Addr:              *address,
		Handler:           requestLog(handler),
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Printf("golf web app listening on http://localhost%s (database %s)", *address, *databasePath)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

func spaHandler(apiHandler http.Handler, staticDir string) http.Handler {
	files := http.FileServer(http.Dir(staticDir))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			apiHandler.ServeHTTP(w, r)
			return
		}
		path := filepath.Join(staticDir, filepath.Clean(r.URL.Path))
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			files.ServeHTTP(w, r)
			return
		}
		http.ServeFile(w, r, filepath.Join(staticDir, "index.html"))
	})
}

func requestLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		next.ServeHTTP(w, r)
		if strings.HasPrefix(r.URL.Path, "/api/") {
			log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(started).Round(time.Millisecond))
		}
	})
}

func envOr(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
