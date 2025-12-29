package main

import (
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

	app.Run(":9000")
}
