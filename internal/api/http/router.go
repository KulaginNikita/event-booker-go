package http

import "github.com/wb-go/wbf/ginext"

func NewRouter(handler *Handler) *ginext.Engine {
	engine := ginext.New("release")
	engine.Use(ginext.Logger(), ginext.Recovery())

	engine.GET("/", func(c *ginext.Context) {
		c.File("web/user.html")
	})
	engine.GET("/admin", func(c *ginext.Context) {
		c.File("web/admin.html")
	})
	engine.GET("/healthz", handler.Live)
	engine.GET("/livez", handler.Live)
	engine.GET("/readyz", handler.Ready)

	engine.POST("/events", handler.CreateEvent)
	engine.GET("/events", handler.ListEvents)
	engine.GET("/events/:id", handler.GetEvent)
	engine.POST("/events/:id/book", handler.Book)
	engine.POST("/events/:id/confirm", handler.Confirm)

	return engine
}
