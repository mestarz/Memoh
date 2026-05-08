package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"sync"

	"github.com/labstack/echo/v4"

	"github.com/memohai/memoh/internal/agent/tools/websearch"
	"github.com/memohai/memoh/internal/searchproviders"
)

type SearchProvidersHandler struct {
	service *searchproviders.Service
	logger  *slog.Logger
}

func NewSearchProvidersHandler(log *slog.Logger, service *searchproviders.Service) *SearchProvidersHandler {
	return &SearchProvidersHandler{
		service: service,
		logger:  log.With(slog.String("handler", "search_providers")),
	}
}

func (h *SearchProvidersHandler) Register(e *echo.Echo) {
	group := e.Group("/search-providers")
	group.GET("/meta", h.ListMeta)
	group.POST("", h.Create)
	group.GET("", h.List)
	group.GET("/:id", h.Get)
	group.PUT("/:id", h.Update)
	group.DELETE("/:id", h.Delete)
	group.GET("/:id/probe-keys", h.ProbeKeys)
}

// ListMeta godoc
// @Summary List search provider metadata
// @Description List available search provider types and config schemas
// @Tags search-providers
// @Success 200 {array} searchproviders.ProviderMeta
// @Router /search-providers/meta [get].
func (h *SearchProvidersHandler) ListMeta(c echo.Context) error {
	return c.JSON(http.StatusOK, h.service.ListMeta(c.Request().Context()))
}

// Create godoc
// @Summary Create a search provider
// @Description Create a search provider configuration
// @Tags search-providers
// @Accept json
// @Produce json
// @Param request body searchproviders.CreateRequest true "Search provider configuration"
// @Success 201 {object} searchproviders.GetResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /search-providers [post].
func (h *SearchProvidersHandler) Create(c echo.Context) error {
	var req searchproviders.CreateRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	if strings.TrimSpace(req.Name) == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "name is required")
	}
	if strings.TrimSpace(string(req.Provider)) == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "provider is required")
	}
	resp, err := h.service.Create(c.Request().Context(), req)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return c.JSON(http.StatusCreated, resp)
}

// List godoc
// @Summary List search providers
// @Description List configured search providers
// @Tags search-providers
// @Accept json
// @Produce json
// @Param provider query string false "Provider filter (brave)"
// @Success 200 {array} searchproviders.GetResponse
// @Failure 500 {object} ErrorResponse
// @Router /search-providers [get].
func (h *SearchProvidersHandler) List(c echo.Context) error {
	items, err := h.service.List(c.Request().Context(), c.QueryParam("provider"))
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return c.JSON(http.StatusOK, items)
}

// Get godoc
// @Summary Get a search provider
// @Description Get search provider by ID
// @Tags search-providers
// @Accept json
// @Produce json
// @Param id path string true "Provider ID"
// @Success 200 {object} searchproviders.GetResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /search-providers/{id} [get].
func (h *SearchProvidersHandler) Get(c echo.Context) error {
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "id is required")
	}
	resp, err := h.service.Get(c.Request().Context(), id)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, err.Error())
	}
	return c.JSON(http.StatusOK, resp)
}

// Update godoc
// @Summary Update a search provider
// @Description Update search provider by ID
// @Tags search-providers
// @Accept json
// @Produce json
// @Param id path string true "Provider ID"
// @Param request body searchproviders.UpdateRequest true "Updated configuration"
// @Success 200 {object} searchproviders.GetResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /search-providers/{id} [put].
func (h *SearchProvidersHandler) Update(c echo.Context) error {
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "id is required")
	}
	var req searchproviders.UpdateRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	resp, err := h.service.Update(c.Request().Context(), id, req)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return c.JSON(http.StatusOK, resp)
}

// Delete godoc
// @Summary Delete a search provider
// @Description Delete search provider by ID
// @Tags search-providers
// @Accept json
// @Produce json
// @Param id path string true "Provider ID"
// @Success 204 "No Content"
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /search-providers/{id} [delete].
func (h *SearchProvidersHandler) Delete(c echo.Context) error {
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "id is required")
	}
	if err := h.service.Delete(c.Request().Context(), id); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return c.NoContent(http.StatusNoContent)
}

// ProbeKeys godoc
// @Summary Probe Tavily API key usage
// @Description Query usage stats for all Tavily API keys in a provider's pool. Only supported for Tavily providers.
// @Tags search-providers
// @Produce json
// @Param id path string true "Provider ID"
// @Success 200 {array} websearch.TavilyKeyUsage
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /search-providers/{id}/probe-keys [get].
func (h *SearchProvidersHandler) ProbeKeys(c echo.Context) error {
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "id is required")
	}
	row, err := h.service.GetRawByID(c.Request().Context(), id)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, err.Error())
	}
	if row.Provider != "tavily" {
		return echo.NewHTTPError(http.StatusBadRequest, "probe-keys is only supported for Tavily providers")
	}

	var cfg map[string]any
	if len(row.Config) > 0 {
		_ = json.Unmarshal(row.Config, &cfg)
	}

	pool := websearch.TavilyAPIKeyPool(cfg)
	if len(pool) == 0 {
		if v, _ := cfg["api_key"].(string); strings.TrimSpace(v) != "" {
			pool = []string{v}
		}
	}

	endpoint := "https://api.tavily.com/search"
	if v, _ := cfg["base_url"].(string); strings.TrimSpace(v) != "" {
		endpoint = v
	}

	type probeJob struct {
		idx int
		key string
	}
	jobs := make([]probeJob, len(pool))
	for i, k := range pool {
		jobs[i] = probeJob{idx: i, key: k}
	}

	results := make([]websearch.TavilyKeyUsage, len(pool))
	var wg sync.WaitGroup
	for _, job := range jobs {
		wg.Add(1)
		go func(j probeJob) {
			defer wg.Done()
			usage := websearch.QueryKeyUsage(c.Request().Context(), j.key, endpoint)
			usage.Index = j.idx
			usage.MaskedKey = websearch.MaskAPIKey(j.key)
			results[j.idx] = usage
		}(job)
	}
	wg.Wait()

	return c.JSON(http.StatusOK, results)
}
