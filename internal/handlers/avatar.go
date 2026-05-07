package handlers

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/labstack/echo/v4"
)

// AvatarHandler provides image upload and serving for user/bot avatars.
type AvatarHandler struct {
	dir    string
	logger *slog.Logger
}

// NewAvatarHandler creates an AvatarHandler that stores avatars in dataRoot/avatars.
func NewAvatarHandler(log *slog.Logger, dataRoot string) *AvatarHandler {
	if log == nil {
		log = slog.Default()
	}
	dir := filepath.Join(dataRoot, "avatars")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		log.Warn("avatar: failed to create avatar directory", slog.String("dir", dir), slog.Any("error", err))
	}
	return &AvatarHandler{
		dir:    dir,
		logger: log.With(slog.String("handler", "avatar")),
	}
}

func (h *AvatarHandler) Register(e *echo.Echo) {
	e.POST("/avatars/upload", h.Upload)
	e.GET("/avatars/:filename", h.Serve)
}

type avatarUploadResponse struct {
	URL string `json:"url"`
}

var allowedAvatarTypes = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/gif":  ".gif",
	"image/webp": ".webp",
}

// Upload godoc
// @Summary Upload an avatar image
// @Description Upload a JPEG/PNG/GIF/WebP image as an avatar; returns its serving URL.
// @Tags avatars
// @Accept multipart/form-data
// @Produce json
// @Param image formData file true "Image file (JPEG/PNG/GIF/WebP, max 5 MB)"
// @Success 200 {object} avatarUploadResponse
// @Router /avatars/upload [post].
func (h *AvatarHandler) Upload(c echo.Context) error {
	file, err := c.FormFile("image")
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "image field is required")
	}

	const maxSize = 5 * 1024 * 1024 // 5 MB
	if file.Size > maxSize {
		return echo.NewHTTPError(http.StatusBadRequest, "file too large (max 5 MB)")
	}

	contentType := file.Header.Get("Content-Type")
	mt, _, _ := mime.ParseMediaType(contentType)
	ext, ok := allowedAvatarTypes[mt]
	if !ok {
		return echo.NewHTTPError(http.StatusBadRequest, "unsupported image type: "+mt)
	}

	src, err := file.Open()
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to open file")
	}
	defer src.Close()

	// Spool into a temp file while computing SHA-256 hash.
	h256 := sha256.New()
	tmp, err := os.CreateTemp("", "avatar-*")
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "temp file error")
	}
	defer func() {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
	}()

	if _, err := io.Copy(io.MultiWriter(h256, tmp), src); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to read file")
	}

	hash := hex.EncodeToString(h256.Sum(nil))[:20]
	filename := hash + ext
	dest := filepath.Join(h.dir, filename) //nolint:gosec // path is constructed from trusted hash+extension

	// Dedup: skip write if identical content already stored.
	if _, statErr := os.Stat(dest); os.IsNotExist(statErr) {
		if _, err := tmp.Seek(0, io.SeekStart); err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "seek error")
		}
		out, err := os.Create(dest) //nolint:gosec
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to save file")
		}
		defer out.Close()
		if _, err := io.Copy(out, tmp); err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to write file")
		}
	}

	return c.JSON(http.StatusOK, avatarUploadResponse{URL: "/avatars/" + filename})
}

// Serve streams a stored avatar image.
func (h *AvatarHandler) Serve(c echo.Context) error {
	filename := filepath.Base(c.Param("filename"))
	if strings.ContainsAny(filename, "/\\") || strings.Contains(filename, "..") || filename == "." {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid filename")
	}
	fp := filepath.Join(h.dir, filename) //nolint:gosec
	ext := filepath.Ext(filename)
	mt := mime.TypeByExtension(ext)
	if mt == "" {
		mt = "application/octet-stream"
	}
	c.Response().Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	return c.File(fp)
}
