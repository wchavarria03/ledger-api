package httpserver

import (
	"github.com/gin-gonic/gin"

	"ledger-api/app/internal/handlers"
	"ledger-api/app/internal/http/middleware"
)

// NewRouter creates a new Router with all routes configured.
func NewRouter(hdlrs *handlers.Registry, jwksURL string, allowedOrigins []string) *Router {
	engine := gin.New()
	engine.Use(gin.Recovery())
	engine.Use(gin.Logger())
	engine.Use(middleware.CORS(allowedOrigins))

	setupRoutes(engine, hdlrs, jwksURL)

	return &Router{engine: engine}
}

// setupRoutes configures all versioned routes for the application.
func setupRoutes(engine *gin.Engine, hdlrs *handlers.Registry, jwksURL string) {
	v1 := engine.Group("/v1")
	v1.Use(middleware.Auth(jwksURL))

	v1.GET("/me", hdlrs.Me.GetMe)
	setupAccountRoutes(v1, hdlrs)
	setupCategoryRoutes(v1, hdlrs)
	setupReportRoutes(v1, hdlrs)
	v1.POST("/import", hdlrs.Upload.Import)
}

func setupAccountRoutes(rg *gin.RouterGroup, hdlrs *handlers.Registry) {
	accounts := rg.Group("/accounts")
	accounts.GET("", hdlrs.Account.List)
	accounts.POST("", hdlrs.Account.Create)
	accounts.GET("/:id", hdlrs.Account.Get)
	accounts.PATCH("/:id", hdlrs.Account.Update)
	accounts.GET("/:id/transactions", hdlrs.Transaction.ListByAccount)
	accounts.GET("/:id/rule-exceptions", hdlrs.RuleException.ListByAccount)
	accounts.POST("/:id/rule-exceptions", hdlrs.RuleException.Disable)
	accounts.DELETE("/:id/rule-exceptions/:rule_id", hdlrs.RuleException.Enable)
}

func setupReportRoutes(rg *gin.RouterGroup, hdlrs *handlers.Registry) {
	reports := rg.Group("/reports")
	reports.GET("/summary", hdlrs.Report.GetSummary)
}

func setupCategoryRoutes(rg *gin.RouterGroup, hdlrs *handlers.Registry) {
	cats := rg.Group("/categories")
	cats.GET("", hdlrs.Category.List)
	cats.POST("", hdlrs.Category.Create)
	cats.PATCH("/:id", hdlrs.Category.Update)
	cats.DELETE("/:id", hdlrs.Category.Delete)

	rules := rg.Group("/category-rules")
	rules.GET("", hdlrs.Category.ListRules)
	rules.POST("", hdlrs.Category.CreateRule)
	rules.DELETE("/:id", hdlrs.Category.DeleteRule)

	txs := rg.Group("/transactions")
	txs.PATCH("/:id/categories", hdlrs.Category.SetTransactionCategories)
}
