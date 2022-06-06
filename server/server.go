package main

import (
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"log"
)

func main() {
	app := fiber.New()
	app.Use(logger.New())
	app.Static("/", "../web/public")
	log.Fatalln(app.Listen("localhost:8080"))
}
