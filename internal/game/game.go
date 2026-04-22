package game

import (
	"image/color"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"claude-pixel/internal/anim"
	"claude-pixel/internal/config"
	"claude-pixel/internal/debug"
	"claude-pixel/internal/input"
	"claude-pixel/internal/player"
	"claude-pixel/internal/world"
)

type Game struct {
	cfg        *config.Config
	world      *world.World
	player     *player.Player
	overlay    *debug.Overlay
	lastIntent input.Intent
}

func New(cfg *config.Config, anims map[string]*anim.Animation, physics *player.Physics, dbgCfg *debug.Config) *Game {
	w := world.New(cfg, physics.Gravity)
	p := player.New(player.Config{
		StartX:  float64(cfg.WindowW) / 2,
		StartY:  w.GroundY,
		Physics: physics,
		Anims:   anims,
	})
	p.Grounded = true
	p.HasAirDash = true

	g := &Game{cfg: cfg, world: w, player: p}
	g.overlay = debug.NewOverlay(dbgCfg, g)
	return g
}

func (g *Game) Player() *player.Player { return g.player }
func (g *Game) Intent() *input.Intent  { return &g.lastIntent }
func (g *Game) EngineFPS() float64     { return ebiten.ActualFPS() }
func (g *Game) EngineTPS() float64     { return ebiten.ActualTPS() }

func (g *Game) Layout(outerW, outerH int) (int, int) { return g.cfg.WindowW, g.cfg.WindowH }

func (g *Game) Update() error {
	if inpututil.IsKeyJustPressed(ebiten.KeyF3) {
		g.overlay.Toggle()
	}
	g.lastIntent = input.Poll()
	dt := time.Second / 60
	g.player.FSM.Handle(g.player, g.lastIntent, dt)
	g.player.ApplyPhysics(g.world, dt)
	if g.player.Current != nil {
		g.player.Current.Update(dt)
	}
	return nil
}

func (g *Game) Draw(screen *ebiten.Image) {
	screen.Fill(color.RGBA{0x80, 0x80, 0x80, 0xFF})

	vector.DrawFilledRect(screen, 0, float32(g.world.GroundY), float32(g.cfg.WindowW), float32(g.cfg.WindowH)-float32(g.world.GroundY),
		color.RGBA{0x3A, 0x3A, 0x3A, 0xFF}, false)

	if g.player.Current != nil && g.player.Current.CurrentFrame() != nil {
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(-float64(g.cfg.SpriteFrameW)/2, -float64(g.cfg.SpriteFrameH))
		if g.player.Facing < 0 {
			op.GeoM.Scale(-1, 1)
		}
		op.GeoM.Scale(float64(g.cfg.RenderScale), float64(g.cfg.RenderScale))
		op.GeoM.Translate(g.player.X, g.player.Y)
		op.Filter = ebiten.FilterNearest
		screen.DrawImage(g.player.Current.CurrentFrame(), op)
	}

	g.overlay.Draw(screen)
}
