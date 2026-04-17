package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/contextops/api/internal/db"
	"github.com/contextops/api/internal/handler"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func main() {
	zerolog.TimeFieldFormat = time.RFC3339
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339})

	port := getEnv("PORT", "8080")
	dbURL := getEnv("DATABASE_URL", "postgres://contextops:contextops@localhost:5432/contextops?sslmode=disable")
	redisURL := getEnv("REDIS_URL", "redis://localhost:6379/0")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pool, err := db.NewPool(ctx, dbURL)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect to database")
	}
	defer pool.Close()

	_ = redisURL // Redis will be used for async eval queue in Phase 2

	r := setupRouter(pool)

	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      r,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	go func() {
		log.Info().Str("port", port).Msg("starting ContextOps API")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("server error")
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info().Msg("shutting down server...")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatal().Err(err).Msg("server forced to shutdown")
	}
	log.Info().Msg("server stopped")
}

func setupRouter(pool *db.Pool) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(requestLogger())
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization", "X-Tenant-ID"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	h := handler.New(pool)

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "service": "contextops-api", "version": "0.1.0"})
	})

	v1 := r.Group("/api/v1")
	v1.Use(tenantMiddleware())
	{
		// Tenants
		v1.POST("/tenants", h.CreateTenant)
		v1.GET("/tenants", h.ListTenants)
		v1.GET("/tenants/:id", h.GetTenant)

		// Identities
		v1.POST("/identities", h.CreateIdentity)
		v1.GET("/identities", h.ListIdentities)
		v1.GET("/identities/:id", h.GetIdentity)

		// Agents
		v1.POST("/agents", h.CreateAgent)
		v1.GET("/agents", h.ListAgents)
		v1.GET("/agents/:id", h.GetAgent)

		// Runs (core)
		v1.POST("/runs", h.IngestRun)
		v1.GET("/runs", h.ListRuns)
		v1.GET("/runs/:id", h.GetRun)
		v1.GET("/runs/:id/timeline", h.GetRunTimeline)
		v1.GET("/runs/:id/context-manifest", h.GetContextManifest)

		// Evaluations
		v1.POST("/runs/:id/evaluate", h.TriggerEvaluation)
		v1.GET("/runs/:id/evaluations", h.GetRunEvaluations)
		v1.GET("/evaluations", h.ListEvaluations)

		// Benchmarks
		v1.POST("/benchmarks", h.CreateBenchmarkSuite)
		v1.GET("/benchmarks", h.ListBenchmarkSuites)
		v1.GET("/benchmarks/:id", h.GetBenchmarkSuite)
		v1.POST("/benchmarks/:id/run", h.RunBenchmark)
		v1.GET("/benchmarks/:id/results", h.GetBenchmarkResults)

		// Traces (canonical format adapter)
		v1.POST("/traces", h.IngestTrace)

		// Datasets
		v1.POST("/datasets", h.CreateDataset)
		v1.GET("/datasets", h.ListDatasets)
		v1.GET("/datasets/:id", h.GetDataset)

		// Compare
		v1.POST("/compare", h.CompareRuns)

		// Audit
		v1.GET("/audit", h.ListAuditLog)
	}

	return r
}

func tenantMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := c.GetHeader("X-Tenant-ID")
		if tenantID == "" {
			tenantID = "00000000-0000-0000-0000-000000000001" // default tenant
		}
		c.Set("tenant_id", tenantID)
		c.Next()
	}
}

func requestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		log.Info().
			Str("method", c.Request.Method).
			Str("path", c.Request.URL.Path).
			Int("status", c.Writer.Status()).
			Dur("latency", time.Since(start)).
			Msg("request")
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// suppress unused import
var _ = fmt.Sprintf
