package server

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/DoraTiger/doratiger-counter/internal/config"
	"github.com/DoraTiger/doratiger-counter/internal/handler"
)

func New(counterHandler *handler.CounterHandler, cfg *config.Config) *gin.Engine {
	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery())

	if cfg.Counter.EnableCors {
		router.Use(func(c *gin.Context) {
			origin := c.GetHeader("Origin")
			if origin != "" && cfg.Counter.IsOriginAllowed(origin) {
				c.Header("Access-Control-Allow-Origin", origin)
				c.Header("Vary", "Origin")
			}
			c.Next()
		})
	}

	router.GET("/count", counterHandler.Count)
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	return router
}
