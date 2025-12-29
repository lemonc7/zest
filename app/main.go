package main

import (
	"fmt"
	"net/http"

	"github.com/lemonc7/engx"
)

func main() {
	app := engx.New()
	api := app.Group("/api")
	api.GET("/hello", func(c *engx.Context) error {
		return c.String(http.StatusOK, "hello")
	})
	api.GET("/users/{name}", func(c *engx.Context) error {
		return c.JSON(http.StatusOK, engx.Map{
			"name": c.Param("name"),
		})
	})

	api.GET("/temp/{path...}", func(c *engx.Context) error {
		return c.HTML(http.StatusOK, fmt.Sprintf("<h1>path: %s</h1>", c.Param("path")))
	})

	app.Run(":9000")
}
