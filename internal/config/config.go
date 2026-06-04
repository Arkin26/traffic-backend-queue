package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	HTTPAddr          string
	DatabaseURL       string
	RedisURL          string
	JWTSecret         string
	AdminAPIKey       string
	DemoAPIKey        string
	SimulationBaseURL string
	HoldTTL           time.Duration
	AdmissionTTL      time.Duration
	MaxTicketsPerUser int
	DBMaxConns        int
	DBMinConns        int
	DemoAutoSetup     bool
	LoadtestMaxVUs    int
	LoadtestMaxQueue  int
	LoadtestMaxConcurrent int
	LoadtestMaxSteadySec int
	LoadtestDefaultVUs int
}

func Load() Config {
	holdMin, _ := strconv.Atoi(getEnv("HOLD_TTL_MINUTES", "12"))
	maxSteadySec, _ := strconv.Atoi(getEnv("LOADTEST_MAX_STEADY_SEC", "90"))
	return Config{
		HTTPAddr:          getEnv("HTTP_ADDR", ":8080"),
		DatabaseURL:       getEnv("DATABASE_URL", "postgres://traffic:traffic@localhost:5432/traffic?sslmode=disable"),
		RedisURL:          getEnv("REDIS_URL", "redis://localhost:6379/0"),
		JWTSecret:         getEnv("JWT_SECRET", "dev-secret-change-in-production"),
		AdminAPIKey:       getEnv("ADMIN_API_KEY", "dev-admin-key"),
		DemoAPIKey:        getEnv("DEMO_API_KEY", ""),
		SimulationBaseURL: getEnv("SIMULATION_BASE_URL", "http://127.0.0.1:8080"),
		HoldTTL:           time.Duration(holdMin) * time.Minute,
		AdmissionTTL:      60 * time.Second,
		MaxTicketsPerUser: 4,
		DBMaxConns:        intEnv("DB_MAX_CONNS", 50),
		DBMinConns:        intEnv("DB_MIN_CONNS", 5),
		DemoAutoSetup:     boolEnv("DEMO_AUTO_SETUP", true),
		LoadtestMaxVUs:    intEnv("LOADTEST_MAX_VUS", 150),
		LoadtestMaxQueue:  intEnv("LOADTEST_MAX_QUEUE", 3),
		LoadtestMaxConcurrent: intEnv("LOADTEST_MAX_CONCURRENT", 1),
		LoadtestMaxSteadySec: maxSteadySec,
		LoadtestDefaultVUs: intEnv("LOADTEST_DEFAULT_VUS", 80),
	}
}

func intEnv(k string, def int) int {
	if v := os.Getenv(k); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func boolEnv(k string, def bool) bool {
	v := os.Getenv(k)
	if v == "" {
		return def
	}
	switch strings.ToLower(v) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return def
	}
}

func getEnv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
