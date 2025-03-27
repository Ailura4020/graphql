package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

func main() {
	InitServer()
}

func InitServer() {
	server := NewServer(":8080",
		10*time.Second,
		10*time.Second,
		30*time.Second,
		2*time.Second,
	)

	// Add routes for your GraphQL proxy and auth
	server.Handle("/api/graphql", graphqlProxyHandler)
	server.Handle("/api/auth/signin", authHandler)

	if err := server.Start(); err != nil {
		fmt.Printf("Error starting server: %v\n", err)
	}
}

// Add this function to proxy GraphQL requests
func graphqlProxyHandler(w http.ResponseWriter, r *http.Request) {
	// Extract JWT from Authorization header
	authHeader := r.Header.Get("Authorization")
	if !strings.HasPrefix(authHeader, "Bearer ") {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Configure the proxy to the external GraphQL endpoint
	graphqlEndpoint, _ := url.Parse("https://DOMAIN/api/graphql-engine/v1/graphql")
	proxy := httputil.NewSingleHostReverseProxy(graphqlEndpoint)

	// Forward the request
	r.URL.Host = graphqlEndpoint.Host
	r.URL.Scheme = graphqlEndpoint.Scheme
	r.Host = graphqlEndpoint.Host
	proxy.ServeHTTP(w, r)
}

// Add authentication handler
func authHandler(w http.ResponseWriter, r *http.Request) {
	// Implement authentication logic here
	// Parse Basic auth, validate credentials
	// Generate JWT if valid
}

// Add CORS middleware to your server
type Server struct {
	port              string
	routes            []Route
	readTimeout       time.Duration
	writeTimeout      time.Duration
	idleTimeout       time.Duration
	readHeaderTimeout time.Duration
}

func NewServer(port string, readTimeout, writeTimeout, idleTimeout, readHeaderTimeout time.Duration) *Server {
	return &Server{
		port:              port,
		routes:            []Route{},
		readTimeout:       readTimeout,
		writeTimeout:      writeTimeout,
		idleTimeout:       idleTimeout,
		readHeaderTimeout: readHeaderTimeout,
	}
}

type Route struct {
	Path    string
	Handler http.HandlerFunc
}

func (s *Server) Handle(path string, handler http.HandlerFunc) {
	s.routes = append(s.routes, Route{Path: path, Handler: handler})
}

func (s *Server) Start() error {
	mux := http.NewServeMux()

	for _, route := range s.routes {
		// Add CORS middleware
		handler := corsMiddleware(route.Handler)
		mux.HandleFunc(route.Path, handler)
	}

	fs := http.FileServer(http.Dir("static"))
	mux.Handle("/static/", http.StripPrefix("/static/", fs))

	server := &http.Server{
		Addr:              s.port,
		Handler:           mux,
		ReadTimeout:       s.readTimeout,
		WriteTimeout:      s.writeTimeout,
		IdleTimeout:       s.idleTimeout,
		ReadHeaderTimeout: s.readHeaderTimeout,
	}

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-done
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			fmt.Printf("Server shutdown error: %v\n", err)
		}
	}()

	fmt.Printf("Starting HTTP server on http://localhost%s\n", s.port)
	return server.ListenAndServe()
}

// CORS middleware function
func corsMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next(w, r)
	}
}
