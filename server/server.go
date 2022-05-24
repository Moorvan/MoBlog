package main

import (
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"log"
)

func main() {
	app := fiber.New()
	app.Use(logger.New())
	app.Static("/", "../hugoWeb/public")
	log.Fatalln(app.ListenTLS(":80", "/etc/letsencrypt/live/morvan.dev/fullchain.pem", "/etc/letsencrypt/live/morvan.dev/privkey.pem"))
	//log.Fatalln(app.Listen(":80"))
}
