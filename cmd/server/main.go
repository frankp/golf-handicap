package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"golf/internal/api"
	"golf/internal/auth"
	"golf/internal/database"
)

func main() {
	log.SetFlags(log.Ltime)
	address := flag.String("addr", envOr("GOLF_ADDR", ":8080"), "HTTP listen address")
	databasePath := flag.String("db", envOr("GOLF_DB", "golf.db"), "SQLite database path")
	staticDir := flag.String("static", envOr("GOLF_STATIC", "web/dist"), "built frontend directory")
	flag.Parse()

	authentication, err := auth.New(auth.Config{
		Password:     os.Getenv("GOLF_ADMIN_PASSWORD"),
		PasswordHash: os.Getenv("GOLF_ADMIN_PASSWORD_HASH"),
		SecureCookie: envBool("GOLF_COOKIE_SECURE", true),
	})
	if err != nil {
		log.Fatal(err)
	}

	store, err := database.Open(*databasePath)
	if err != nil {
		log.Fatal(err)
	}
	defer store.Close()

	apiHandler := api.New(store, authentication)
	handler := spaHandler(apiHandler, *staticDir)
	server := &http.Server{
		Addr:              *address,
		Handler:           requestLog(handler),
		ReadHeaderTimeout: 5 * time.Second,
	}
	signalContext, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-signalContext.Done()
		shutdownContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownContext); err != nil {
			log.Printf("server shutdown: %v", err)
		}
	}()

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

func envBool(name string, fallback bool) bool {
	value := os.Getenv(name)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		log.Fatalf("%s must be true or false", name)
	}
	return parsed
}
