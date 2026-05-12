package provider

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/memohai/memoh/internal/config"
	containerapi "github.com/memohai/memoh/internal/container"
	dockeradapter "github.com/memohai/memoh/internal/container/docker"
)

// ProvideService creates the workspace container Service. Only the Docker
// backend is supported; other backends were removed when the server moved to
// a host-process deployment model.
func ProvideService(_ context.Context, log *slog.Logger, cfg config.Config, backend string) (containerapi.Service, func(), error) {
	if backend != containerapi.BackendDocker {
		return nil, nil, fmt.Errorf("unsupported container backend %q (only %q is supported)", backend, containerapi.BackendDocker)
	}
	svc, err := dockeradapter.NewService(log, cfg)
	if err != nil {
		return nil, nil, err
	}
	return svc, func() { _ = svc.Close() }, nil
}
