package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"sync"
	"time"

	pb "github.com/gorgio/network/api/proto"
	"github.com/gorgio/network/pkg/validator"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
)

type URLServiceServer struct {
	pb.UnimplementedURLServiceServer
	redis   *redis.Client
	storage map[string]*URLData
	mu      sync.RWMutex
	baseURL string
}

type URLData struct {
	ShortCode   string
	OriginalURL string
	UserID      string
	CreatedAt   int64
}

func NewURLServiceServer(redisClient *redis.Client) *URLServiceServer {
	domain := os.Getenv("DOMAIN_NAME")
	baseURL := "http://localhost:8080"

	if domain != "" && domain != "localhost" {
		// For production domains, use https
		baseURL = "https://" + domain
	}

	server := &URLServiceServer{
		redis:   redisClient,
		storage: make(map[string]*URLData),
		baseURL: baseURL,
	}

	// Restore data from Redis
	ctx := context.Background()
	iter := redisClient.Scan(ctx, 0, "urldata:*", 0).Iterator()
	count := 0
	
	for iter.Next(ctx) {
		key := iter.Val()
		val, err := redisClient.Get(ctx, key).Result()
		if err != nil {
			log.Printf("Failed to load key %s: %v", key, err)
			continue
		}

		var urlData URLData
		if err := json.Unmarshal([]byte(val), &urlData); err != nil {
			log.Printf("Failed to unmarshal key %s: %v", key, err)
			continue
		}

		server.storage[urlData.ShortCode] = &urlData
		count++
	}

	if err := iter.Err(); err != nil {
		log.Printf("Error during Redis scan: %v", err)
	}

	log.Printf("Restored %d URLs from Redis storage", count)

	return server
}

func (s *URLServiceServer) CreateShortURL(ctx context.Context, req *pb.CreateShortURLRequest) (*pb.CreateShortURLResponse, error) {
	log.Printf("CreateShortURL request: original_url=%s, user_id=%s, custom_alias=%s",
		req.OriginalUrl, req.UserId, req.CustomAlias)

	// Validate URL
	if err := validator.ValidateURL(req.OriginalUrl); err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	var shortCode string
	if req.CustomAlias != "" {
		// Use custom alias if provided
		if err := validator.ValidateShortCode(req.CustomAlias); err != nil {
			return nil, fmt.Errorf("invalid custom alias: %w", err)
		}
		shortCode = req.CustomAlias

		// Check if alias already exists
		s.mu.RLock()
		_, exists := s.storage[shortCode]
		s.mu.RUnlock()
		if exists {
			return nil, fmt.Errorf("alias already exists")
		}
	} else {
		// Generate random short code
		shortCode = generateShortCode()
	}

	createdAt := time.Now().Unix()
	urlData := &URLData{
		ShortCode:   shortCode,
		OriginalURL: req.OriginalUrl,
		UserID:      req.UserId,
		CreatedAt:   createdAt,
	}

	// Store in memory
	s.mu.Lock()
	s.storage[shortCode] = urlData
	s.mu.Unlock()

	// 1. Persist full data in Redis (Permanent storage)
	jsonData, err := json.Marshal(urlData)
	if err != nil {
		log.Printf("Failed to marshal URL data: %v", err)
	} else {
		persistKey := fmt.Sprintf("urldata:%s", shortCode)
		err := s.redis.Set(ctx, persistKey, jsonData, 0).Err() // 0 = no expiration
		if err != nil {
			log.Printf("Failed to persist in Redis: %v", err)
		}
	}

	// 2. Cache simple mapping in Redis (Fast lookups, 24h TTL)
	cacheKey := fmt.Sprintf("url:%s", shortCode)
	err = s.redis.Set(ctx, cacheKey, req.OriginalUrl, 24*time.Hour).Err()
	if err != nil {
		log.Printf("Failed to cache in Redis: %v", err)
	}

	log.Printf("Created short URL: %s -> %s", shortCode, req.OriginalUrl)

	return &pb.CreateShortURLResponse{
		ShortCode:   shortCode,
		ShortUrl:    fmt.Sprintf("%s/s/%s", s.baseURL, shortCode),
		OriginalUrl: req.OriginalUrl,
		CreatedAt:   createdAt,
	}, nil
}

func (s *URLServiceServer) GetOriginalURL(ctx context.Context, req *pb.GetOriginalURLRequest) (*pb.GetOriginalURLResponse, error) {
	log.Printf("GetOriginalURL request: short_code=%s", req.ShortCode)

	// Try Redis cache first
	cacheKey := fmt.Sprintf("url:%s", req.ShortCode)
	cachedURL, err := s.redis.Get(ctx, cacheKey).Result()
	if err == nil {
		log.Printf("Cache hit for %s: %s", req.ShortCode, cachedURL)
		return &pb.GetOriginalURLResponse{
			OriginalUrl: cachedURL,
			Found:       true,
		}, nil
	}

	// Check in-memory storage
	s.mu.RLock()
	urlData, exists := s.storage[req.ShortCode]
	s.mu.RUnlock()

	if !exists {
		log.Printf("Short code not found: %s", req.ShortCode)
		return &pb.GetOriginalURLResponse{
			Found: false,
		}, nil
	}

	// Update Redis cache
	s.redis.Set(ctx, cacheKey, urlData.OriginalURL, 24*time.Hour)

	log.Printf("Found URL: %s -> %s", req.ShortCode, urlData.OriginalURL)

	return &pb.GetOriginalURLResponse{
		OriginalUrl: urlData.OriginalURL,
		Found:       true,
	}, nil
}

func (s *URLServiceServer) GetUserURLs(ctx context.Context, req *pb.GetUserURLsRequest) (*pb.GetUserURLsResponse, error) {
	log.Printf("GetUserURLs request: user_id=%s", req.UserId)

	// Initialize as empty slice to avoid null in JSON
	urls := make([]*pb.URLInfo, 0)

	s.mu.RLock()
	for _, urlData := range s.storage {
		if urlData.UserID == req.UserId {
			urls = append(urls, &pb.URLInfo{
				ShortCode:   urlData.ShortCode,
				ShortUrl:    fmt.Sprintf("%s/s/%s", s.baseURL, urlData.ShortCode),
				OriginalUrl: urlData.OriginalURL,
				CreatedAt:   urlData.CreatedAt,
				Clicks:      0, // Will be populated from analytics
			})
		}
	}
	s.mu.RUnlock()

	log.Printf("Found %d URLs for user %s", len(urls), req.UserId)

	return &pb.GetUserURLsResponse{
		Urls: urls,
	}, nil
}

func generateShortCode() string {
	b := make([]byte, 6)
	rand.Read(b)
	code := base64.URLEncoding.EncodeToString(b)
	// Take first 6 characters and replace special chars
	code = code[:6]
	return code
}

func main() {
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

	// Create gRPC server
	lis, err := net.Listen("tcp", ":8081")
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	grpcServer := grpc.NewServer()
	pb.RegisterURLServiceServer(grpcServer, NewURLServiceServer(redisClient))

	log.Println("URL Service started on :8081")
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}
