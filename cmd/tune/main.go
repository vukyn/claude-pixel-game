package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"text/tabwriter"

	"github.com/urfave/cli/v3"

	"claude-pixel/internal/config"
	"claude-pixel/internal/player"
	"claude-pixel/internal/storage"
)

func main() {
	cfg := config.Load()
	db := storage.MustOpen(cfg)
	defer db.Close()

	repo := storage.NewRepository[player.TuningParam](db, player.TuningMapper{})

	app := &cli.Command{
		Name:  "claude-pixel-tune",
		Usage: "Update physics tuning values stored in SQLite",
		Commands: []*cli.Command{
			{
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
			},
			{
				Name:      "set",
				Usage:     "Update an existing parameter (no creation)",
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
			},
		},
	}

	if err := app.Run(context.Background(), os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
