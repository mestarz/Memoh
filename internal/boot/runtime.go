package boot

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/memohai/memoh/internal/config"
	"github.com/memohai/memoh/internal/timezone"
)

type RuntimeConfig struct {
	JwtSecret        string `json:"-"`
	JwtExpiresIn     time.Duration
	ServerAddr       string
	ContainerBackend string // only "docker" is supported
	Timezone         string
	TimezoneLocation *time.Location
}

func ProvideRuntimeConfig(cfg config.Config) (*RuntimeConfig, error) {
	if strings.TrimSpace(cfg.Auth.JWTSecret) == "" {
		return nil, errors.New("jwt secret is required")
	}

	jwtExpiresIn, err := time.ParseDuration(cfg.Auth.JWTExpiresIn)
	if err != nil {
		return nil, fmt.Errorf("invalid jwt expires in: %w", err)
	}

	backend := normalizeContainerBackend(cfg.Container.Backend)
	if backend == "" {
		return nil, errors.New(`container backend is required; set [container].backend = "docker"`)
	}
	if backend != "docker" {
		return nil, fmt.Errorf(`unsupported container backend %q; only "docker" is supported`, backend)
	}

	tzName := strings.TrimSpace(cfg.Timezone)
	if envTZ := strings.TrimSpace(os.Getenv("TZ")); envTZ != "" {
		tzName = envTZ
	}
	tzLocation, resolvedTZ, err := timezone.Resolve(tzName)
	if err != nil {
		return nil, err
	}

	ret := &RuntimeConfig{
		JwtSecret:        cfg.Auth.JWTSecret,
		JwtExpiresIn:     jwtExpiresIn,
		ServerAddr:       cfg.Server.Addr,
		ContainerBackend: backend,
		Timezone:         resolvedTZ,
		TimezoneLocation: tzLocation,
	}

	if value := os.Getenv("HTTP_ADDR"); value != "" {
		ret.ServerAddr = value
	}
	return ret, nil
}

func normalizeContainerBackend(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}
