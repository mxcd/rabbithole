package server

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mxcd/rabbithole/internal/util"
)

func (s *Server) registerHealthRoute() {
	s.Engine.GET(s.Options.ApiBaseUrl+"/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "ok",
			"version": util.Version,
		})
	})
}
