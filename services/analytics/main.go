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
	now := time.Now()

	clickData := &ClickData{
		ShortCode: shortCode,
		IPAddress: ipAddress,
		UserAgent: userAgent,
		Referer:   referer,
		Timestamp: now.Unix(),
	}

	s.mu.Lock()
	s.clicks[shortCode] = append(s.clicks[shortCode], clickData)
	s.mu.Unlock()

	totalKey := fmt.Sprintf("clicks:total:%s", shortCode)
	uniqueKey := fmt.Sprintf("clicks:unique:%s", shortCode)
	dateKey := fmt.Sprintf("clicks:daily:%s:%s", shortCode, now.Format("2006-01-02"))
	hourKey := fmt.Sprintf("clicks:hourly:%s:%s", shortCode, now.Format("2006-01-02-15"))
	refererKey := fmt.Sprintf("clicks:referers:%s", shortCode)
	globalKey := "clicks:global:sorted"
	globalWeekKey := fmt.Sprintf("clicks:global:week:%s", now.Format("2006-W01"))
	globalMonthKey := fmt.Sprintf("clicks:global:month:%s", now.Format("2006-01"))

	pipe := s.redis.Pipeline()
	pipe.Incr(ctx, totalKey)
	pipe.SAdd(ctx, uniqueKey, req.IpAddress)
	pipe.Incr(ctx, dateKey)
	pipe.Incr(ctx, hourKey)

	if referer != "" {
		pipe.ZIncrBy(ctx, refererKey, 1, referer)
		pipe.Expire(ctx, refererKey, 30*24*time.Hour)
	}

	pipe.ZIncrBy(ctx, globalKey, 1, shortCode)
	pipe.ZIncrBy(ctx, globalWeekKey, 1, shortCode)
	pipe.ZIncrBy(ctx, globalMonthKey, 1, shortCode)

	pipe.Expire(ctx, totalKey, 30*24*time.Hour)
	pipe.Expire(ctx, uniqueKey, 30*24*time.Hour)
	pipe.Expire(ctx, dateKey, 30*24*time.Hour)
	pipe.Expire(ctx, hourKey, 30*24*time.Hour)
	pipe.Expire(ctx, globalWeekKey, 90*24*time.Hour)
	pipe.Expire(ctx, globalMonthKey, 180*24*time.Hour)

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

func (s *AnalyticsServiceServer) GetTopURLs(ctx context.Context, req *pb.GetTopURLsRequest) (*pb.GetTopURLsResponse, error) {
	log.Printf("GetTopURLs: period=%s, limit=%d", req.Period, req.Limit)

	var key string
	now := time.Now()

	switch req.Period {
	case "all":
		key = "clicks:global:sorted"
	case "week":
		key = fmt.Sprintf("clicks:global:week:%s", now.Format("2006-W01"))
	case "month":
		key = fmt.Sprintf("clicks:global:month:%s", now.Format("2006-01"))
	default:
		key = "clicks:global:sorted"
	}

	limit := req.Limit
	if limit <= 0 || limit > 100 {
		limit = 100
	}

	results, err := s.redis.ZRevRangeWithScores(ctx, key, 0, int64(limit-1)).Result()
	if err != nil {
		log.Printf("Failed to get top URLs: %v", err)
		return &pb.GetTopURLsResponse{Urls: []*pb.TopURLItem{}}, nil
	}

	var urls []*pb.TopURLItem
	for _, result := range results {
		urls = append(urls, &pb.TopURLItem{
			ShortCode: result.Member.(string),
			Clicks:    int64(result.Score),
		})
	}

	log.Printf("Found %d top URLs for period %s", len(urls), req.Period)

	return &pb.GetTopURLsResponse{Urls: urls}, nil
}

func (s *AnalyticsServiceServer) GetTopReferers(ctx context.Context, req *pb.GetTopReferersRequest) (*pb.GetTopReferersResponse, error) {
	log.Printf("GetTopReferers: short_code=%s, limit=%d", req.ShortCode, req.Limit)

	refererKey := fmt.Sprintf("clicks:referers:%s", req.ShortCode)

	limit := req.Limit
	if limit <= 0 || limit > 100 {
		limit = 100
	}

	results, err := s.redis.ZRevRangeWithScores(ctx, refererKey, 0, int64(limit-1)).Result()
	if err != nil {
		log.Printf("Failed to get top referers: %v", err)
		return &pb.GetTopReferersResponse{Referers: []*pb.RefererItem{}}, nil
	}

	var referers []*pb.RefererItem
	for _, result := range results {
		referers = append(referers, &pb.RefererItem{
			Referer: result.Member.(string),
			Count:   int64(result.Score),
		})
	}

	log.Printf("Found %d top referers for %s", len(referers), req.ShortCode)

	return &pb.GetTopReferersResponse{Referers: referers}, nil
}

func (s *AnalyticsServiceServer) GetHourlyDistribution(ctx context.Context, req *pb.GetHourlyDistributionRequest) (*pb.GetHourlyDistributionResponse, error) {
	log.Printf("GetHourlyDistribution: short_code=%s, date=%s", req.ShortCode, req.Date)

	date := req.Date
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}

	var hours []*pb.HourlyClick
	for hour := 0; hour < 24; hour++ {
		hourKey := fmt.Sprintf("clicks:hourly:%s:%s-%02d", req.ShortCode, date, hour)
		count, err := s.redis.Get(ctx, hourKey).Int64()
		if err != nil {
			count = 0
		}
		hours = append(hours, &pb.HourlyClick{
			Hour:  int32(hour),
			Count: count,
		})
	}

	log.Printf("Retrieved hourly distribution for %s on %s", req.ShortCode, date)

	return &pb.GetHourlyDistributionResponse{Hours: hours}, nil
}

func main() {
	redisClient := redis.NewClient(&redis.Options{
		Addr: "redis:6379",
	})

	ctx := context.Background()
	if err := redisClient.Ping(ctx).Err(); err != nil {
		log.Printf("Warning: Redis connection failed: %v", err)
	} else {
		log.Println("Connected to Redis")
	}

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
