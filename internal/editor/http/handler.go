package http

import (
	"github.com/gofiber/fiber/v2"

	"claude-pixel/internal/editor/service"
)

type Deps struct {
	Behavior *service.Behavior
	Tuning   *service.Tuning
	Registry *service.Registry
}

func Register(app *fiber.App, d Deps) {
	app.Get("/api/health", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{"ok": true}) })

	app.Get("/api/behaviors", func(c *fiber.Ctx) error {
		refs, err := d.Behavior.List()
		if err != nil {
			return fiber.NewError(500, err.Error())
		}
		return c.JSON(refs)
	})
	app.Get("/api/behaviors/:kind", func(c *fiber.Ctx) error {
		raw, err := d.Behavior.Get(c.Params("kind"))
		if err != nil {
			return fiber.NewError(404, err.Error())
		}
		c.Set("Content-Type", "application/json")
		return c.Send(raw)
	})
	app.Put("/api/behaviors/:kind", func(c *fiber.Ctx) error {
		body := c.Body()
		if err := d.Behavior.Update(c.Params("kind"), body); err != nil {
			return c.Status(400).JSON(d.Behavior.Validate(c.Params("kind"), body))
		}
		return c.JSON(fiber.Map{"ok": true})
	})
	app.Post("/api/behaviors/:kind/validate", func(c *fiber.Ctx) error {
		return c.JSON(d.Behavior.Validate(c.Params("kind"), c.Body()))
	})

	app.Get("/api/tuning", func(c *fiber.Ctx) error {
		rows, err := d.Tuning.List(c.Query("prefix"))
		if err != nil {
			return fiber.NewError(500, err.Error())
		}
		return c.JSON(rows)
	})
	app.Put("/api/tuning/:key", func(c *fiber.Ctx) error {
		var body struct {
			Value float64 `json:"value"`
		}
		if err := c.BodyParser(&body); err != nil {
			return fiber.NewError(400, "body must be {\"value\": number}")
		}
		old, err := d.Tuning.Update(c.Params("key"), body.Value)
		if err != nil {
			return fiber.NewError(400, err.Error())
		}
		return c.JSON(fiber.Map{"ok": true, "old": old, "new": body.Value})
	})

	app.Get("/api/registry/actions", func(c *fiber.Ctx) error { return c.JSON(d.Registry.Actions()) })
	app.Get("/api/registry/conditions", func(c *fiber.Ctx) error {
		return c.JSON(d.Registry.Conditions())
	})
}
