package handlers

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/memohai/memoh/internal/appsettings"
	"github.com/memohai/memoh/internal/httpproxy"
)

// AppSettingsHandler exposes the global app-level settings API (currently
// limited to network/proxy configuration).
type AppSettingsHandler struct {
	service *appsettings.Service
	logger  *slog.Logger
}

// NewAppSettingsHandler constructs an AppSettingsHandler.
func NewAppSettingsHandler(log *slog.Logger, service *appsettings.Service) *AppSettingsHandler {
	if log == nil {
		log = slog.Default()
	}
	return &AppSettingsHandler{
		service: service,
		logger:  log.With(slog.String("handler", "app_settings")),
	}
}

// AppNetworkSettings is the response/request payload for the network section
// of the app-wide settings.
type AppNetworkSettings struct {
	HTTPProxyURL string `json:"http_proxy_url"`
}

// Register registers the app settings routes onto the given Echo instance.
func (h *AppSettingsHandler) Register(e *echo.Echo) {
	group := e.Group("/app-settings")
	group.GET("/network", h.GetNetwork)
	group.PUT("/network", h.PutNetwork)
	group.POST("/network/test", h.TestNetwork)
}

// GetNetwork godoc
// @Summary Get network/proxy settings
// @Description Returns the global HTTP proxy URL applied to providers that opted in.
// @Tags app-settings
// @Success 200 {object} AppNetworkSettings
// @Failure 500 {object} ErrorResponse
// @Router /app-settings/network [get].
func (h *AppSettingsHandler) GetNetwork(c echo.Context) error {
	if h.service == nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "app settings service unavailable")
	}
	url, err := h.service.GetHTTPProxyURL(c.Request().Context())
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return c.JSON(http.StatusOK, AppNetworkSettings{HTTPProxyURL: url})
}

// PutNetwork godoc
// @Summary Update network/proxy settings
// @Description Sets the global HTTP proxy URL. An empty value clears the proxy.
// @Tags app-settings
// @Param payload body AppNetworkSettings true "Network settings payload"
// @Success 200 {object} AppNetworkSettings
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /app-settings/network [put].
func (h *AppSettingsHandler) PutNetwork(c echo.Context) error {
	if h.service == nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "app settings service unavailable")
	}
	var payload AppNetworkSettings
	if err := c.Bind(&payload); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	normalized, err := h.service.SetHTTPProxyURL(c.Request().Context(), payload.HTTPProxyURL)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	return c.JSON(http.StatusOK, AppNetworkSettings{HTTPProxyURL: normalized})
}

// AppNetworkTestRequest configures the proxy connectivity test.
type AppNetworkTestRequest struct {
	HTTPProxyURL string `json:"http_proxy_url"`
	TargetURL    string `json:"target_url,omitempty"`
}

// AppNetworkTestResponse describes the result of a proxy connectivity test.
type AppNetworkTestResponse struct {
	OK         bool   `json:"ok"`
	StatusCode int    `json:"status_code,omitempty"`
	LatencyMs  int64  `json:"latency_ms"`
	TargetURL  string `json:"target_url"`
	Error      string `json:"error,omitempty"`
}

// TestNetwork godoc
// @Summary Test the configured (or supplied) HTTP proxy
// @Description Issues an HTTP GET against target_url through the proxy and reports outcome.
// @Tags app-settings
// @Param payload body AppNetworkTestRequest true "Proxy test payload"
// @Success 200 {object} AppNetworkTestResponse
// @Failure 400 {object} ErrorResponse
// @Router /app-settings/network/test [post].
func (h *AppSettingsHandler) TestNetwork(c echo.Context) error {
	var payload AppNetworkTestRequest
	if err := c.Bind(&payload); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	proxyURL := strings.TrimSpace(payload.HTTPProxyURL)
	if proxyURL == "" && h.service != nil {
		current, err := h.service.GetHTTPProxyURL(c.Request().Context())
		if err == nil {
			proxyURL = current
		}
	}

	target := strings.TrimSpace(payload.TargetURL)
	if target == "" {
		target = "https://www.google.com/generate_204"
	}

	resp := AppNetworkTestResponse{TargetURL: target}

	transport := &http.Transport{TLSHandshakeTimeout: 10 * time.Second}
	if err := httpproxy.ApplyToTransport(transport, proxyURL); err != nil {
		resp.Error = err.Error()
		return c.JSON(http.StatusOK, resp)
	}

	client := &http.Client{Transport: transport, Timeout: 10 * time.Second}
	ctx, cancel := context.WithTimeout(c.Request().Context(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		resp.Error = err.Error()
		return c.JSON(http.StatusOK, resp)
	}

	start := time.Now()
	httpResp, err := client.Do(req) //nolint:gosec // intentional outbound probe — admin endpoint validating an admin-supplied proxy
	resp.LatencyMs = time.Since(start).Milliseconds()
	if err != nil {
		resp.Error = err.Error()
		return c.JSON(http.StatusOK, resp)
	}
	defer func() { _ = httpResp.Body.Close() }()

	resp.StatusCode = httpResp.StatusCode
	resp.OK = httpResp.StatusCode >= 200 && httpResp.StatusCode < 400
	if !resp.OK {
		resp.Error = httpResp.Status
	}
	return c.JSON(http.StatusOK, resp)
}
