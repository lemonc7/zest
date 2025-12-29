package main

import (
	"fmt"
	"net/http"

	"github.com/lemonc7/engx"
)

func main() {
	app := engx.New()
	app.GET("/", func(c *engx.Context) error {
		return c.String(http.StatusOK, "root")
	})

	app.GET("/hello", func(c *engx.Context) error {
		return c.String(http.StatusOK, "hello")
	})

	app.GET("/hello/:name", func(c *engx.Context) error {
		return c.String(http.StatusOK, fmt.Sprintf("hello %s\n", c.Param("name")))
	})

	app.GET("/assets/*filepath", func(c *engx.Context) error {
		return c.String(http.StatusOK, fmt.Sprintf("filepath: %v", c.Param("filepath")))
	})

	app.Run(":9000")
}
