package core

import (
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"log"
	"server/tags"
)

func RunServer() {
	app := fiber.New()
	app.Use(logger.New())
	app.Static("/", "../hugoWeb/public")
	if tags.Debug {
		log.Fatalln(app.Listen(":8080"))
	} else {
		log.Fatalln(app.ListenTLS(":443", "/etc/letsencrypt/live/morvan.dev/fullchain.pem", "/etc/letsencrypt/live/morvan.dev/privkey.pem"))
	}
}
