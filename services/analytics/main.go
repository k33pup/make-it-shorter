package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	pb "github.com/gorgio/network/api/proto"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
)

type AnalyticsServiceServer struct {
	pb.UnimplementedAnalyticsServiceServer
	redis   *redis.Client
	clicks  map[string][]*ClickData
	mu      sync.RWMutex
}

type ClickData struct {
	ShortCode string
	IPAddress string
	UserAgent string
	Referer   string
	Timestamp int64
}

func NewAnalyticsServiceServer(redisClient *redis.Client) *AnalyticsServiceServer {
	return &AnalyticsServiceServer{
		redis:  redisClient,
		clicks: make(map[string][]*ClickData),
	}
}

func (s *AnalyticsServiceServer) RecordClick(ctx context.Context, req *pb.RecordClickRequest) (*pb.RecordClickResponse, error) {
	log.Printf("RecordClick: short_code=%s, ip=%s", req.ShortCode, req.IpAddress)

	shortCode := req.ShortCode
	ipAddress := req.IpAddress[:min(len(req.IpAddress), 45)]
	userAgent := req.UserAgent[:min(len(req.UserAgent), 500)]
	referer := req.Referer[:min(len(req.Referer), 500)]

	clickData := &ClickData{
		ShortCode: shortCode,
		IPAddress: ipAddress,
		UserAgent: userAgent,
		Referer:   referer,
		Timestamp: time.Now().Unix(),
	}

	s.mu.Lock()
	s.clicks[shortCode] = append(s.clicks[shortCode], clickData)
	s.mu.Unlock()

	totalKey := fmt.Sprintf("clicks:total:%s", shortCode)
	uniqueKey := fmt.Sprintf("clicks:unique:%s", shortCode)
	dateKey := fmt.Sprintf("clicks:daily:%s:%s", shortCode, time.Now().Format("2006-01-02"))

	pipe := s.redis.Pipeline()
	pipe.Incr(ctx, totalKey)
	pipe.SAdd(ctx, uniqueKey, req.IpAddress)
	pipe.Incr(ctx, dateKey)
	pipe.Expire(ctx, totalKey, 30*24*time.Hour)
	pipe.Expire(ctx, uniqueKey, 30*24*time.Hour)
	pipe.Expire(ctx, dateKey, 30*24*time.Hour)

	_, err := pipe.Exec(ctx)
	if err != nil {
		log.Printf("Failed to record click in Redis: %v", err)
	}

	log.Printf("Click recorded for %s", req.ShortCode)

	return &pb.RecordClickResponse{
		Success: true,
	}, nil
}

func (s *AnalyticsServiceServer) GetClickStats(ctx context.Context, req *pb.GetClickStatsRequest) (*pb.GetClickStatsResponse, error) {
	log.Printf("GetClickStats: short_code=%s", req.ShortCode)

	totalKey := fmt.Sprintf("clicks:total:%s", req.ShortCode)
	uniqueKey := fmt.Sprintf("clicks:unique:%s", req.ShortCode)

	// Get total clicks
	totalClicks, err := s.redis.Get(ctx, totalKey).Int64()
	if err != nil {
		if err.Error() != "redis: nil" {
			log.Printf("Failed to get total clicks: %v", err)
		}
		totalClicks = 0
	}

	uniqueClicks, err := s.redis.SCard(ctx, uniqueKey).Result()
	if err != nil {
		if err.Error() != "redis: nil" {
			log.Printf("Failed to get unique clicks: %v", err)
		}
		uniqueClicks = 0
	}

	// Get daily clicks for last 7 days
	var dailyClicks []*pb.DailyClick
	for i := 0; i < 7; i++ {
		date := time.Now().AddDate(0, 0, -i).Format("2006-01-02")
		dateKey := fmt.Sprintf("clicks:daily:%s:%s", req.ShortCode, date)
		count, err := s.redis.Get(ctx, dateKey).Int64()
		if err != nil {
			count = 0
		}
		dailyClicks = append(dailyClicks, &pb.DailyClick{
			Date:  date,
			Count: count,
		})
	}

	stats := &pb.ClickStats{
		TotalClicks:  totalClicks,
		UniqueClicks: uniqueClicks,
		DailyClicks:  dailyClicks,
	}

	log.Printf("Stats for %s: total=%d, unique=%d", req.ShortCode, totalClicks, uniqueClicks)

	return &pb.GetClickStatsResponse{
		Stats: stats,
	}, nil
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
	lis, err := net.Listen("tcp", ":8082")
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	grpcServer := grpc.NewServer()
	pb.RegisterAnalyticsServiceServer(grpcServer, NewAnalyticsServiceServer(redisClient))

	log.Println("Analytics Service started on :8082")
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}
