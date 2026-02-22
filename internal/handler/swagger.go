package handler

import (
	"github.com/gin-gonic/gin"
)

// SwaggerUI serves the Swagger UI HTML page
// @Summary Swagger UI
// @Description Interactive API documentation
// @Tags documentation
// @Produce html
// @Success 200 {string} string "Swagger UI HTML page"
// @Router /docs [get]
func SwaggerUI(c *gin.Context) {
	c.File("./docs/swagger-ui.html")
}

// SwaggerJSON serves the Swagger JSON specification
// @Summary Swagger JSON
// @Description Swagger API specification
// @Tags documentation
// @Produce json
// @Success 200 {string} string "Swagger JSON specification"
// @Router /docs/swagger.json [get]
func SwaggerJSON(c *gin.Context) {
	c.File("./docs/swagger.json")
}

// Add Swagger routes to the router
func AddSwaggerRoutes(router *gin.Engine) {
	router.StaticFile("/docs", "./docs/swagger-ui.html")
	router.StaticFile("/docs/swagger.json", "./docs/swagger.json")
	
	// Also serve at /swagger.json for compatibility
	router.GET("/swagger.json", func(c *gin.Context) {
		c.File("./docs/swagger.json")
	})
}
