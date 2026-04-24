package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"text/tabwriter"

	"github.com/urfave/cli/v3"

	"claude-pixel/internal/combat"
	"claude-pixel/internal/config"
	"claude-pixel/internal/hud"
	"claude-pixel/internal/player"
	"claude-pixel/internal/storage"
)

func main() {
	cfg := config.Load()
	db := storage.MustOpen(cfg)
	defer db.Close()

	tuneRepo := storage.NewRepository[player.TuningParam](db, player.TuningMapper{})
	hbRepo := storage.NewRepository[combat.HitboxSpec](db, combat.HitboxMapper{})
	hudRepo := storage.NewRepository[hud.LayoutRow](db, hud.LayoutMapper{})

	app := &cli.Command{
		Name:  "claude-pixel-tune",
		Usage: "Manage tuning + hitbox rows stored in SQLite",
		Commands: []*cli.Command{
			tuningListCmd(tuneRepo),
			tuningSetCmd(tuneRepo),
			hitboxesCmd(hbRepo),
			hudCmd(hudRepo),
		},
	}

	if err := app.Run(context.Background(), os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func tuningListCmd(repo *storage.Repository[player.TuningParam]) *cli.Command {
	return &cli.Command{
		Name:  "list",
		Usage: "List every tunable parameter",
		Action: func(ctx context.Context, c *cli.Command) error {
			params, err := repo.List(ctx)
			if err != nil {
				return err
			}
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "KEY\tVALUE\tMIN\tMAX\tUNIT\tDESCRIPTION")
			for _, p := range params {
				fmt.Fprintf(w, "%s\t%.2f\t%.2f\t%.2f\t%s\t%s\n",
					p.Key, p.Value, p.MinValue, p.MaxValue, p.Unit, p.Description)
			}
			return w.Flush()
		},
	}
}

func tuningSetCmd(repo *storage.Repository[player.TuningParam]) *cli.Command {
	return &cli.Command{
		Name:      "set",
		Usage:     "Update an existing tuning parameter (no creation)",
		ArgsUsage: "<key> <value>",
		Action: func(ctx context.Context, c *cli.Command) error {
			if c.Args().Len() != 2 {
				return fmt.Errorf("usage: tune set <key> <value>")
			}
			key := c.Args().Get(0)
			raw := c.Args().Get(1)

			p, err := repo.Get(ctx, key)
			if err != nil {
				return fmt.Errorf("unknown tuning key %q. Run \"tune list\" to see valid keys", key)
			}

			newVal, err := strconv.ParseFloat(raw, 64)
			if err != nil {
				return fmt.Errorf("value %q is not a number: %v", raw, err)
			}

			if err := player.ValidateTuning(p, newVal); err != nil {
				return err
			}

			old := p.Value
			p.Value = newVal
			if err := repo.Upsert(ctx, p); err != nil {
				return err
			}
			unit := ""
			if p.Unit != "" {
				unit = " " + p.Unit
			}
			fmt.Printf("OK: %s = %.4f%s (was %.4f)\n", p.Key, newVal, unit, old)
			return nil
		},
	}
}

func hitboxesCmd(repo *storage.Repository[combat.HitboxSpec]) *cli.Command {
	return &cli.Command{
		Name:  "hitboxes",
		Usage: "CRUD operations on the hitboxes table",
		Commands: []*cli.Command{
			{
				Name:  "list",
				Usage: "List every hitbox row",
				Action: func(ctx context.Context, c *cli.Command) error {
					rows, err := repo.List(ctx)
					if err != nil {
						return err
					}
					w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
					fmt.Fprintln(w, "ID\tOWNER\tKIND\tOFFSET_X\tOFFSET_Y\tWIDTH\tHEIGHT\tFRAME_START\tFRAME_END")
					for _, h := range rows {
						fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%d\t%d\t%d\t%d\t%d\n",
							h.ID, h.Owner, h.Kind, h.OffsetX, h.OffsetY, h.Width, h.Height, h.FrameStart, h.FrameEnd)
					}
					return w.Flush()
				},
			},
			{
				Name:      "get",
				Usage:     "Show one hitbox row by id",
				ArgsUsage: "<id>",
				Action: func(ctx context.Context, c *cli.Command) error {
					if c.Args().Len() != 1 {
						return fmt.Errorf("usage: tune hitboxes get <id>")
					}
					id := c.Args().Get(0)
					h, err := repo.Get(ctx, id)
					if err != nil {
						return fmt.Errorf("unknown hitbox id %q", id)
					}
					fmt.Printf("id=%s owner=%s kind=%s offset_x=%d offset_y=%d width=%d height=%d frame_start=%d frame_end=%d\n",
						h.ID, h.Owner, h.Kind, h.OffsetX, h.OffsetY, h.Width, h.Height, h.FrameStart, h.FrameEnd)
					return nil
				},
			},
			{
				Name:      "set",
				Usage:     "Update one field of an existing hitbox",
				ArgsUsage: "<id> <field> <value>",
				Description: "Valid fields: owner, kind, offset_x, offset_y, width, height, active_frame_start, active_frame_end",
				Action: func(ctx context.Context, c *cli.Command) error {
					if c.Args().Len() != 3 {
						return fmt.Errorf("usage: tune hitboxes set <id> <field> <value>")
					}
					id := c.Args().Get(0)
					field := c.Args().Get(1)
					raw := c.Args().Get(2)

					h, err := repo.Get(ctx, id)
					if err != nil {
						return fmt.Errorf("unknown hitbox id %q", id)
					}

					before := formatHitbox(h)
					if err := applyHitboxField(&h, field, raw); err != nil {
						return err
					}
					if err := repo.Upsert(ctx, h); err != nil {
						return err
					}
					fmt.Printf("OK: %s.%s updated\n  was: %s\n  now: %s\n", id, field, before, formatHitbox(h))
					return nil
				},
			},
			{
				Name:      "add",
				Usage:     "Insert (upsert) a hitbox row",
				ArgsUsage: "<id> <owner> <kind> <offset_x> <offset_y> <width> <height> <frame_start> <frame_end>",
				Description: "frame_start/frame_end = -1 means always active (body boxes)",
				Action: func(ctx context.Context, c *cli.Command) error {
					if c.Args().Len() != 9 {
						return fmt.Errorf("usage: tune hitboxes add <id> <owner> <kind> <off_x> <off_y> <w> <h> <fs> <fe>")
					}
					h := combat.HitboxSpec{
						ID:    c.Args().Get(0),
						Owner: c.Args().Get(1),
						Kind:  c.Args().Get(2),
					}
					ints := []struct {
						idx int
						p   *int
						lbl string
					}{
						{3, &h.OffsetX, "offset_x"},
						{4, &h.OffsetY, "offset_y"},
						{5, &h.Width, "width"},
						{6, &h.Height, "height"},
						{7, &h.FrameStart, "frame_start"},
						{8, &h.FrameEnd, "frame_end"},
					}
					for _, v := range ints {
						n, err := strconv.Atoi(c.Args().Get(v.idx))
						if err != nil {
							return fmt.Errorf("%s=%q is not an integer", v.lbl, c.Args().Get(v.idx))
						}
						*v.p = n
					}
					if err := repo.Upsert(ctx, h); err != nil {
						return err
					}
					fmt.Printf("OK: added/updated %s\n", h.ID)
					return nil
				},
			},
			{
				Name:      "delete",
				Usage:     "Delete a hitbox row by id",
				ArgsUsage: "<id>",
				Action: func(ctx context.Context, c *cli.Command) error {
					if c.Args().Len() != 1 {
						return fmt.Errorf("usage: tune hitboxes delete <id>")
					}
					id := c.Args().Get(0)
					if _, err := repo.Get(ctx, id); err != nil {
						return fmt.Errorf("unknown hitbox id %q", id)
					}
					if err := repo.Delete(ctx, id); err != nil {
						return err
					}
					fmt.Printf("OK: deleted %s\n", id)
					return nil
				},
			},
		},
	}
}

func formatHitbox(h combat.HitboxSpec) string {
	return fmt.Sprintf("owner=%s kind=%s off=(%d,%d) w=%d h=%d frames=[%d,%d]",
		h.Owner, h.Kind, h.OffsetX, h.OffsetY, h.Width, h.Height, h.FrameStart, h.FrameEnd)
}

func applyHitboxField(h *combat.HitboxSpec, field, raw string) error {
	asInt := func() (int, error) {
		n, err := strconv.Atoi(raw)
		if err != nil {
			return 0, fmt.Errorf("value %q is not an integer", raw)
		}
		return n, nil
	}
	switch field {
	case "owner":
		h.Owner = raw
	case "kind":
		h.Kind = raw
	case "offset_x":
		n, err := asInt()
		if err != nil {
			return err
		}
		h.OffsetX = n
	case "offset_y":
		n, err := asInt()
		if err != nil {
			return err
		}
		h.OffsetY = n
	case "width":
		n, err := asInt()
		if err != nil {
			return err
		}
		h.Width = n
	case "height":
		n, err := asInt()
		if err != nil {
			return err
		}
		h.Height = n
	case "active_frame_start", "frame_start":
		n, err := asInt()
		if err != nil {
			return err
		}
		h.FrameStart = n
	case "active_frame_end", "frame_end":
		n, err := asInt()
		if err != nil {
			return err
		}
		h.FrameEnd = n
	default:
		return fmt.Errorf("unknown field %q (valid: owner, kind, offset_x, offset_y, width, height, active_frame_start, active_frame_end)", field)
	}
	return nil
}

func hudCmd(repo *storage.Repository[hud.LayoutRow]) *cli.Command {
	return &cli.Command{
		Name:  "hud",
		Usage: "CRUD operations on the hud_layout table",
		Commands: []*cli.Command{
			{
				Name:  "list",
				Usage: "List every hud_layout row",
				Action: func(ctx context.Context, c *cli.Command) error {
					rows, err := repo.List(ctx)
					if err != nil {
						return err
					}
					w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
					fmt.Fprintln(w, "KEY\tX\tY\tW\tH\tANCHOR\tSCALE")
					for _, r := range rows {
						fmt.Fprintf(w, "%s\t%d\t%d\t%d\t%d\t%s\t%.2f\n",
							r.Key, r.X, r.Y, r.W, r.H, r.AnchorS, r.Scale)
					}
					return w.Flush()
				},
			},
			{
				Name:      "get",
				Usage:     "Show one hud_layout row",
				ArgsUsage: "<key>",
				Action: func(ctx context.Context, c *cli.Command) error {
					if c.Args().Len() != 1 {
						return fmt.Errorf("usage: tune hud get <key>")
					}
					key := c.Args().Get(0)
					r, err := repo.Get(ctx, key)
					if err != nil {
						return fmt.Errorf("unknown hud layout key %q", key)
					}
					fmt.Printf("key=%s x=%d y=%d w=%d h=%d anchor=%s scale=%.2f\n",
						r.Key, r.X, r.Y, r.W, r.H, r.AnchorS, r.Scale)
					return nil
				},
			},
			{
				Name:        "set",
				Usage:       "Update one field of a hud_layout row",
				ArgsUsage:   "<key> <field> <value>",
				Description: "Valid fields: x, y, w, h, anchor, scale",
				Action: func(ctx context.Context, c *cli.Command) error {
					if c.Args().Len() != 3 {
						return fmt.Errorf("usage: tune hud set <key> <field> <value>")
					}
					key := c.Args().Get(0)
					field := c.Args().Get(1)
					raw := c.Args().Get(2)

					r, err := repo.Get(ctx, key)
					if err != nil {
						return fmt.Errorf("unknown hud layout key %q", key)
					}
					before := formatHUDRow(r)
					if err := applyHUDField(&r, field, raw); err != nil {
						return err
					}
					if err := repo.Upsert(ctx, r); err != nil {
						return err
					}
					fmt.Printf("OK: %s.%s updated\n  was: %s\n  now: %s\n", key, field, before, formatHUDRow(r))
					return nil
				},
			},
		},
	}
}

func formatHUDRow(r hud.LayoutRow) string {
	return fmt.Sprintf("x=%d y=%d w=%d h=%d anchor=%s scale=%.2f",
		r.X, r.Y, r.W, r.H, r.AnchorS, r.Scale)
}

func applyHUDField(r *hud.LayoutRow, field, raw string) error {
	asInt := func() (int, error) {
		n, err := strconv.Atoi(raw)
		if err != nil {
			return 0, fmt.Errorf("value %q is not an integer", raw)
		}
		return n, nil
	}
	switch field {
	case "x":
		n, err := asInt()
		if err != nil {
			return err
		}
		r.X = n
	case "y":
		n, err := asInt()
		if err != nil {
			return err
		}
		r.Y = n
	case "w":
		n, err := asInt()
		if err != nil {
			return err
		}
		r.W = n
	case "h":
		n, err := asInt()
		if err != nil {
			return err
		}
		r.H = n
	case "anchor":
		valid := map[string]bool{"top_left": true, "top_right": true, "bottom_left": true, "bottom_right": true}
		if !valid[raw] {
			return fmt.Errorf("invalid anchor %q (valid: top_left, top_right, bottom_left, bottom_right)", raw)
		}
		r.AnchorS = raw
	case "scale":
		f, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return fmt.Errorf("value %q is not a number", raw)
		}
		if f <= 0 {
			return fmt.Errorf("scale must be > 0 (got %f)", f)
		}
		r.Scale = f
	default:
		return fmt.Errorf("unknown field %q (valid: x, y, w, h, anchor, scale)", field)
	}
	return nil
}
