package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	pb "github.com/gorgio/network/api/proto"
	"github.com/gorgio/network/pkg/auth"
	"github.com/gorgio/network/pkg/database"
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
	userDB          *database.UserDB
}

func NewGateway(urlConn, analyticsConn *grpc.ClientConn, rateLimiter *middleware.RateLimiter, userDB *database.UserDB) *Gateway {
	return &Gateway{
		urlClient:       pb.NewURLServiceClient(urlConn),
		analyticsClient: pb.NewAnalyticsServiceClient(analyticsConn),
		rateLimiter:     rateLimiter,
		userDB:          userDB,
	}
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; font-src 'self'")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Permissions-Policy", "geolocation=(), microphone=(), camera=()")
		next.ServeHTTP(w, r)
	})
}

func requestSizeLimit(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			next.ServeHTTP(w, r)
		})
	}
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		allowedOrigin := os.Getenv("ALLOWED_ORIGIN")
		if allowedOrigin == "" {
			allowedOrigin = "http://localhost:8080"
		}

		w.Header().Set("Access-Control-Allow-Origin", allowedOrigin)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Allow-Credentials", "true")

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
		Password string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	username := validator.SanitizeInput(req.Username)
	password := validator.SanitizeInput(req.Password)

	if username == "" || password == "" {
		http.Error(w, "Username and password are required", http.StatusBadRequest)
		return
	}

	valid, err := g.userDB.ValidateUser(username, password)
	if err != nil {
		log.Printf("Error validating user: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	if !valid {
		http.Error(w, "Invalid username or password", http.StatusUnauthorized)
		return
	}

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

func (g *Gateway) handleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Email    string `json:"email"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	username := validator.SanitizeInput(req.Username)
	password := validator.SanitizeInput(req.Password)
	email := validator.SanitizeInput(req.Email)

	if username == "" || password == "" {
		http.Error(w, "Username and password are required", http.StatusBadRequest)
		return
	}

	if err := validator.ValidateAlphanumeric(username, 50); err != nil {
		http.Error(w, "Invalid username format", http.StatusBadRequest)
		return
	}

	if len(username) < 3 {
		http.Error(w, "Username must be at least 3 characters", http.StatusBadRequest)
		return
	}

	if len(password) < 6 {
		http.Error(w, "Password must be at least 6 characters", http.StatusBadRequest)
		return
	}

	if err := validator.ValidateEmail(email); err != nil {
		http.Error(w, "Invalid email format", http.StatusBadRequest)
		return
	}

	exists, err := g.userDB.UserExists(username)
	if err != nil {
		log.Printf("Error checking user existence: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	if exists {
		http.Error(w, "Username already taken", http.StatusConflict)
		return
	}

	err = g.userDB.CreateUser(username, password, email)
	if err != nil {
		log.Printf("Error creating user: %v", err)
		http.Error(w, "Failed to create user", http.StatusInternalServerError)
		return
	}

	token, err := auth.GenerateToken(username)
	if err != nil {
		http.Error(w, "User created but failed to generate token", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"token":   token,
		"user_id": username,
		"message": "User created successfully",
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

	req.URL = validator.SanitizeInput(req.URL)
	req.CustomAlias = validator.SanitizeInput(req.CustomAlias)


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

	if err := validator.ValidateShortCode(shortCode); err != nil {
		http.Error(w, "Invalid short code", http.StatusBadRequest)
		return
	}

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

	if err := validator.ValidateShortCode(shortCode); err != nil {
		http.Error(w, "Invalid short code", http.StatusBadRequest)
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
		if idx := strings.Index(xff, ","); idx != -1 {
			return strings.TrimSpace(xff[:idx])
		}
		return strings.TrimSpace(xff)
	}
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}
	ip := r.RemoteAddr
	if idx := strings.LastIndex(ip, ":"); idx != -1 {
		ip = ip[:idx]
	}
	return ip
}

func main() {
	urlConn, err := grpc.Dial("urlservice:8081", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to connect to URL service: %v", err)
	}
	defer urlConn.Close()

	analyticsConn, err := grpc.Dial("analytics:8082", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to connect to Analytics service: %v", err)
	}
	defer analyticsConn.Close()

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL environment variable is not set")
	}

	userDB, err := database.NewUserDB(databaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer userDB.Close()
	log.Println("Connected to PostgreSQL")

	redisClient := redis.NewClient(&redis.Options{
		Addr: "redis:6379",
	})

	ctx := context.Background()
	if err := redisClient.Ping(ctx).Err(); err != nil {
		log.Printf("Warning: Redis connection failed: %v", err)
	} else {
		log.Println("Connected to Redis")
	}

	rateLimiter := middleware.NewRateLimiter(redisClient, 100, time.Minute)

	gateway := NewGateway(urlConn, analyticsConn, rateLimiter, userDB)

	mux := http.NewServeMux()

	mux.HandleFunc("/api/register", gateway.handleRegister)
	mux.HandleFunc("/api/login", gateway.handleLogin)
	mux.HandleFunc("/api/shorten", gateway.handleCreateShortURL)
	mux.HandleFunc("/api/urls", gateway.handleGetUserURLs)
	mux.HandleFunc("/api/stats", gateway.handleGetStats)

	mux.HandleFunc("/s/", gateway.handleRedirect)

	fs := http.FileServer(http.Dir("/app/web/static"))
	mux.Handle("/", fs)

	handler := corsMiddleware(securityHeaders(requestSizeLimit(1024*1024)(rateLimiter.Middleware(mux))))

	log.Println("API Gateway started on :8080")
	if err := http.ListenAndServe(":8080", handler); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
