package handler

import (
	"net/http"

	"github.com/DoraTiger/doratiger-counter/internal/config"
	"github.com/DoraTiger/doratiger-counter/internal/data"
	"github.com/gin-gonic/gin"
)

type CounterHandler struct {
	repos map[string]*data.CounterRepo
	cfg   *config.CounterConfig
}

func NewCounterHandler(repos map[string]*data.CounterRepo, cfg *config.CounterConfig) *CounterHandler {
	return &CounterHandler{repos: repos, cfg: cfg}
}

// Count 处理 GET /count?page=<path>&uid=<uuid>
func (h *CounterHandler) Count(c *gin.Context) {
	// Origin 校验
	requestSource := c.GetHeader("Origin")
	if requestSource == "" {
		requestSource = c.GetHeader("Referer")
	}
	siteKey, ok := h.cfg.ResolveSiteKey(requestSource)
	if !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": "origin not allowed"})
		return
	}
	repo, ok := h.repos[siteKey]
	if !ok {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "counter unavailable"})
		return
	}

	page := c.Query("page")
	if page == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing page parameter"})
		return
	}

	uid := c.Query("uid")

	result, err := repo.Increment(c.Request.Context(), page, uid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "counter unavailable"})
		return
	}

	c.JSON(http.StatusOK, result)
}
