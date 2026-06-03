package http

import "github.com/wb-go/wbf/ginext"

func NewRouter(handler *Handler) *ginext.Engine {
	engine := ginext.New("release")
	engine.Use(ginext.Logger(), ginext.Recovery())

	engine.GET("/", func(c *ginext.Context) {
		c.File("web/index.html")
	})
	engine.GET("/admin", func(c *ginext.Context) {
		c.File("web/index.html")
	})
	engine.GET("/healthz", handler.Live)
	engine.GET("/livez", handler.Live)
	engine.GET("/readyz", handler.Ready)
	engine.POST("/auth/login", handler.Login)

	engine.GET("/events", handler.ListEvents)
	engine.GET("/events/:id", handler.GetEvent)
	engine.POST("/events", handler.RequireRole("admin"), handler.CreateEvent)
	engine.POST("/events/:id/book", handler.RequireRole("user", "admin"), handler.Book)
	engine.POST("/events/:id/confirm", handler.RequireRole("user", "admin"), handler.Confirm)

	return engine
}
