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
	engine.GET("/healthz", func(c *ginext.Context) {
		c.String(200, "ok")
	})

	engine.POST("/events", handler.CreateEvent)
	engine.GET("/events", handler.ListEvents)
	engine.GET("/events/:id", handler.GetEvent)
	engine.POST("/events/:id/book", handler.Book)
	engine.POST("/events/:id/confirm", handler.Confirm)

	return engine
}
