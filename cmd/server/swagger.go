package main

import (
	"github.com/gin-gonic/gin"
	"github.com/jack/golang-short-url-service/internal/config"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// SetupSwagger 配置 Swagger UI 路由（含 Basic Auth 保護）
func SetupSwagger(router *gin.Engine, auth *config.AuthConfig) {
	router.StaticFile("/api/docs/openapi.yaml", "./api/openapi.yaml")

	// 如果沒有設置認證，就禁用 Swagger UI
	if auth.BasicUser == "" || auth.BasicPassword == "" {
		router.GET("/docs/*any", func(c *gin.Context) {
			c.String(403, "Swagger UI is disabled. Set AUTH_BASIC_USER and AUTH_BASIC_PASSWORD to enable.")
		})
		return
	}

	authorized := router.Group("/docs", gin.BasicAuth(gin.Accounts{
		auth.BasicUser: auth.BasicPassword,
	}))

	authorized.GET("/*any", ginSwagger.WrapHandler(swaggerFiles.Handler,
		ginSwagger.URL("/api/docs/openapi.yaml"),
		ginSwagger.DefaultModelsExpandDepth(-1),
		ginSwagger.DocExpansion("list"),
	))
}
