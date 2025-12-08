package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	pb "github.com/gorgio/network/api/proto"
	"github.com/gorgio/network/pkg/auth"
	"github.com/gorgio/network/pkg/middleware"
	"github.com/gorgio/network/pkg/validator"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Gateway struct {
	urlClient       pb.URLServiceClient
	analyticsClient pb.AnalyticsServiceClient
	rateLimiter     *middleware.RateLimiter
}

func NewGateway(urlConn, analyticsConn *grpc.ClientConn, rateLimiter *middleware.RateLimiter) *Gateway {
	return &Gateway{
		urlClient:       pb.NewURLServiceClient(urlConn),
		analyticsClient: pb.NewAnalyticsServiceClient(analyticsConn),
		rateLimiter:     rateLimiter,
	}
}

// Middleware to add security headers
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline'")
		next.ServeHTTP(w, r)
	})
}

// CORS middleware
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (g *Gateway) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Username string `json:"username"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// Sanitize input
	username := validator.SanitizeInput(req.Username)
	if username == "" {
		http.Error(w, "Username is required", http.StatusBadRequest)
		return
	}

	// Generate JWT token
	token, err := auth.GenerateToken(username)
	if err != nil {
		http.Error(w, "Failed to generate token", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"token":   token,
		"user_id": username,
	})
}

func (g *Gateway) handleCreateShortURL(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract JWT token
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		http.Error(w, "Authorization header required", http.StatusUnauthorized)
		return
	}

	tokenString := strings.TrimPrefix(authHeader, "Bearer ")
	claims, err := auth.ValidateToken(tokenString)
	if err != nil {
		http.Error(w, "Invalid token", http.StatusUnauthorized)
		return
	}

	var req struct {
		URL         string `json:"url"`
		CustomAlias string `json:"custom_alias,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// Sanitize input
	req.URL = validator.SanitizeInput(req.URL)
	req.CustomAlias = validator.SanitizeInput(req.CustomAlias)

	// Call URL service via gRPC
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := g.urlClient.CreateShortURL(ctx, &pb.CreateShortURLRequest{
		OriginalUrl: req.URL,
		UserId:      claims.UserID,
		CustomAlias: req.CustomAlias,
	})

	if err != nil {
		log.Printf("Error creating short URL: %v", err)
		http.Error(w, fmt.Sprintf("Failed to create short URL: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (g *Gateway) handleRedirect(w http.ResponseWriter, r *http.Request) {
	shortCode := strings.TrimPrefix(r.URL.Path, "/s/")
	if shortCode == "" {
		http.Error(w, "Short code required", http.StatusBadRequest)
		return
	}

	// Get original URL from URL service
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	urlResp, err := g.urlClient.GetOriginalURL(ctx, &pb.GetOriginalURLRequest{
		ShortCode: shortCode,
	})

	if err != nil {
		log.Printf("Error getting original URL: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	if !urlResp.Found {
		http.Error(w, "Short URL not found", http.StatusNotFound)
		return
	}

	// Record click in analytics service (async)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		_, err := g.analyticsClient.RecordClick(ctx, &pb.RecordClickRequest{
			ShortCode: shortCode,
			IpAddress: getClientIP(r),
			UserAgent: r.UserAgent(),
			Referer:   r.Referer(),
		})
		if err != nil {
			log.Printf("Failed to record click: %v", err)
		}
	}()

	// Redirect to original URL
	http.Redirect(w, r, urlResp.OriginalUrl, http.StatusMovedPermanently)
}

func (g *Gateway) handleGetUserURLs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract JWT token
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		http.Error(w, "Authorization header required", http.StatusUnauthorized)
		return
	}

	tokenString := strings.TrimPrefix(authHeader, "Bearer ")
	claims, err := auth.ValidateToken(tokenString)
	if err != nil {
		http.Error(w, "Invalid token", http.StatusUnauthorized)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := g.urlClient.GetUserURLs(ctx, &pb.GetUserURLsRequest{
		UserId: claims.UserID,
	})

	if err != nil {
		log.Printf("Error getting user URLs: %v", err)
		http.Error(w, "Failed to get URLs", http.StatusInternalServerError)
		return
	}

	// Ensure empty array instead of null
	if resp.Urls == nil {
		resp.Urls = []*pb.URLInfo{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (g *Gateway) handleGetStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	shortCode := r.URL.Query().Get("code")
	if shortCode == "" {
		http.Error(w, "Short code required", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := g.analyticsClient.GetClickStats(ctx, &pb.GetClickStatsRequest{
		ShortCode: shortCode,
	})

	if err != nil {
		log.Printf("Error getting stats: %v", err)
		http.Error(w, "Failed to get stats", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func getClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return xff
	}
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}
	return r.RemoteAddr
}

func main() {
	// Connect to URL service
	urlConn, err := grpc.Dial("urlservice:8081", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to connect to URL service: %v", err)
	}
	defer urlConn.Close()

	// Connect to Analytics service
	analyticsConn, err := grpc.Dial("analytics:8082", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to connect to Analytics service: %v", err)
	}
	defer analyticsConn.Close()

	// Connect to Redis
	redisClient := redis.NewClient(&redis.Options{
		Addr: "redis:6379",
	})

	ctx := context.Background()
	if err := redisClient.Ping(ctx).Err(); err != nil {
		log.Printf("Warning: Redis connection failed: %v", err)
	} else {
		log.Println("Connected to Redis")
	}

	// Create rate limiter (10 requests per minute)
	rateLimiter := middleware.NewRateLimiter(redisClient, 100, time.Minute)

	gateway := NewGateway(urlConn, analyticsConn, rateLimiter)

	// Setup routes
	mux := http.NewServeMux()

	// API routes
	mux.HandleFunc("/api/login", gateway.handleLogin)
	mux.HandleFunc("/api/shorten", gateway.handleCreateShortURL)
	mux.HandleFunc("/api/urls", gateway.handleGetUserURLs)
	mux.HandleFunc("/api/stats", gateway.handleGetStats)

	// Redirect route
	mux.HandleFunc("/s/", gateway.handleRedirect)

	// Serve static files
	fs := http.FileServer(http.Dir("/app/web/static"))
	mux.Handle("/", fs)

	// Apply middleware
	handler := corsMiddleware(securityHeaders(rateLimiter.Middleware(mux)))

	log.Println("API Gateway started on :8080")
	if err := http.ListenAndServe(":8080", handler); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
