package game

import (
	"image/color"
	"math/rand"
	"sort"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"claude-pixel/internal/anim"
	"claude-pixel/internal/combat"
	"claude-pixel/internal/config"
	"claude-pixel/internal/debug"
	"claude-pixel/internal/enemy"
	"claude-pixel/internal/hud"
	"claude-pixel/internal/input"
	"claude-pixel/internal/player"
	"claude-pixel/internal/spawner"
	"claude-pixel/internal/world"
)

type GameState int

const (
	Playing GameState = iota
	GameOverState
)

type Deps struct {
	Cfg          *config.Config
	Anims        map[string]*anim.Animation
	Physics      *player.Physics
	DebugCfg     *debug.Config
	SoldierBoxes map[string]combat.Box
	CombatTuning *combat.Tuning
	OrcAnims     map[string]*anim.Animation
	OrcBoxes     map[string]combat.Box
	OrcTuning    *enemy.Tuning
	HeartAnim    *anim.Animation
	HUDFace      *text.GoTextFace
	OverTitle    *text.GoTextFace
	OverSubtitle *text.GoTextFace
}

type Game struct {
	cfg          *config.Config
	world        *world.World
	player       *player.Player
	enemies      []*enemy.Enemy
	spawner      *spawner.Spawner
	overlay      *debug.Overlay
	hud          *hud.HUD
	gameOver     *hud.GameOver
	state        GameState
	hitboxDebug  bool
	lastIntent   input.Intent
	combatTuning *combat.Tuning
	orcAnims     map[string]*anim.Animation
	orcBoxes     map[string]combat.Box
	orcTuning    *enemy.Tuning
	physics      *player.Physics
	rng          *rand.Rand
}

type livesProvider struct{ p *player.Player }

func (l livesProvider) Lives() int { return l.p.Lives }

func New(d Deps) *Game {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	w := world.New(d.Cfg, d.Physics.Gravity)

	p := player.New(player.Config{
		StartX:     float64(d.Cfg.WindowW) / 2,
		StartY:     w.GroundY,
		Physics:    d.Physics,
		Anims:      d.Anims,
		Boxes:      d.SoldierBoxes,
		StartLives: d.CombatTuning.SoldierMaxLives,
	})
	p.Grounded = true

	g := &Game{
		cfg:          d.Cfg,
		world:        w,
		player:       p,
		combatTuning: d.CombatTuning,
		orcAnims:     d.OrcAnims,
		orcBoxes:     d.OrcBoxes,
		orcTuning:    d.OrcTuning,
		physics:      d.Physics,
		rng:          rng,
		state:        Playing,
	}
	g.overlay = debug.NewOverlay(d.DebugCfg, g)

	orcBodyHalfW := float64(d.OrcBoxes["body"].W) / 2
	spawnXMin := orcBodyHalfW
	spawnXMax := float64(d.Cfg.WindowW) - orcBodyHalfW
	orcSpriteH := float64(100 * d.Cfg.RenderScale)

	g.spawner = spawner.New(spawner.Config{
		MinIntervalS: d.OrcTuning.SpawnMinS,
		MaxIntervalS: d.OrcTuning.SpawnMaxS,
		MaxAlive:     int(d.OrcTuning.MaxAlive),
		SpawnXMin:    spawnXMin,
		SpawnXMax:    spawnXMax,
		SpawnY:       -orcSpriteH,
		RNG:          rng,
		NewEnemy: func(x, y float64) *enemy.Enemy {
			return enemy.New(enemy.Config{
				StartX: x, StartY: y,
				Physics: d.Physics,
				Tuning:  d.OrcTuning,
				Anims:   d.OrcAnims,
				Boxes:   d.OrcBoxes,
				RNG:     rng,
			})
		},
	})

	g.hud = hud.NewHUD(d.HeartAnim, d.HUDFace, livesProvider{p}, d.Cfg.WindowW, 3)
	g.gameOver = hud.NewGameOver(d.OverTitle, d.OverSubtitle, d.Cfg.WindowW, d.Cfg.WindowH)

	return g
}

func (g *Game) Player() *player.Player { return g.player }
func (g *Game) Intent() *input.Intent  { return &g.lastIntent }
func (g *Game) EngineFPS() float64     { return ebiten.ActualFPS() }
func (g *Game) EngineTPS() float64     { return ebiten.ActualTPS() }
func (g *Game) OrcCount() int          { return len(g.enemies) }
func (g *Game) NextSpawnS() float64    { return g.spawner.NextSpawnS() }

func (g *Game) Layout(outerW, outerH int) (int, int) { return g.cfg.WindowW, g.cfg.WindowH }

func (g *Game) Update() error {
	if inpututil.IsKeyJustPressed(ebiten.KeyF3) {
		g.overlay.Toggle()
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyF4) {
		g.hitboxDebug = !g.hitboxDebug
	}

	if g.state == GameOverState {
		if inpututil.IsKeyJustPressed(ebiten.KeyR) {
			g.reset()
		}
		return nil
	}

	g.lastIntent = input.Poll()
	dt := time.Second / 60

	g.player.FSM.Handle(g.player, g.lastIntent, dt)
	for _, e := range g.enemies {
		e.FSM.Handle(e, dt)
	}

	g.player.ApplyPhysics(g.world, dt)
	for _, e := range g.enemies {
		e.ApplyPhysics(g.world, dt)
	}

	// Boundary: clamp by body hitbox half-width (post-scale) so sprite transparency
	// doesn't eat into usable play area.
	soldierBodyHalfW := float64(g.player.Boxes["body"].W) / 2
	g.player.X = world.Clamp(g.player.X, soldierBodyHalfW, float64(g.cfg.WindowW)-soldierBodyHalfW)

	for _, e := range g.enemies {
		orcBodyHalfW := float64(e.Boxes["body"].W) / 2
		leftLimit := orcBodyHalfW
		rightLimit := float64(g.cfg.WindowW) - orcBodyHalfW
		clamped := world.Clamp(e.X, leftLimit, rightLimit)
		if clamped != e.X && e.FSM.CurrentID() == enemy.StateRun {
			if e.X <= leftLimit {
				e.Facing = 1
			} else {
				e.Facing = -1
			}
		}
		e.X = clamped
	}

	if g.player.Current != nil {
		g.player.Current.Update(dt)
	}
	for _, e := range g.enemies {
		if e.Current != nil && e.FSM.CurrentID() != enemy.StateFall {
			e.Current.Update(dt)
		}
	}
	g.hud.Update(dt)

	if spawned := g.spawner.Tick(dt, len(g.enemies)); spawned != nil {
		g.enemies = append(g.enemies, spawned)
	}

	g.dispatchSoldierHits()
	g.dispatchOrcHits()

	alive := g.enemies[:0]
	for _, e := range g.enemies {
		if !e.Dead {
			alive = append(alive, e)
		}
	}
	g.enemies = alive

	if g.player.FSM.CurrentID() == player.StateDeath && g.player.Current != nil && g.player.Current.Done() {
		g.state = GameOverState
	}

	return nil
}

func (g *Game) dispatchSoldierHits() {
	attackers := []combat.Fighter{g.player}
	victims := make([]combat.Fighter, 0, len(g.enemies))
	for _, e := range g.enemies {
		victims = append(victims, e)
	}
	for _, ev := range combat.Resolve(attackers, victims) {
		orc := ev.Victim.(*enemy.Enemy)
		orc.OnHit(g.player.X)
	}
}

func (g *Game) dispatchOrcHits() {
	attackers := make([]combat.Fighter, 0, len(g.enemies))
	for _, e := range g.enemies {
		attackers = append(attackers, e)
	}
	victims := []combat.Fighter{g.player}
	for _, ev := range combat.Resolve(attackers, victims) {
		orc := ev.Attacker.(*enemy.Enemy)
		g.player.OnHit(g.combatTuning.SoldierKnockbackVX, g.combatTuning.SoldierKnockbackVY, orc.X)
	}
}

func (g *Game) reset() {
	oldAnims := g.player.Anims
	oldBoxes := g.player.Boxes
	g.enemies = nil
	g.spawner.Reset()
	g.player = player.New(player.Config{
		StartX:     float64(g.cfg.WindowW) / 2,
		StartY:     g.world.GroundY,
		Physics:    g.physics,
		Anims:      oldAnims,
		Boxes:      oldBoxes,
		StartLives: g.combatTuning.SoldierMaxLives,
	})
	g.player.Grounded = true
	g.hud = hud.NewHUD(g.hud.Heart, g.hud.Face, livesProvider{g.player}, g.cfg.WindowW, g.hud.Scale)
	g.state = Playing
}

func (g *Game) Draw(screen *ebiten.Image) {
	screen.Fill(color.RGBA{0x80, 0x80, 0x80, 0xFF})

	vector.DrawFilledRect(screen, 0, float32(g.world.GroundY), float32(g.cfg.WindowW), float32(g.cfg.WindowH)-float32(g.world.GroundY),
		color.RGBA{0x3A, 0x3A, 0x3A, 0xFF}, false)

	enemiesSorted := make([]*enemy.Enemy, len(g.enemies))
	copy(enemiesSorted, g.enemies)
	sort.SliceStable(enemiesSorted, func(i, j int) bool {
		return enemiesSorted[i].Y < enemiesSorted[j].Y
	})
	for _, e := range enemiesSorted {
		g.drawEnemy(screen, e)
	}

	g.drawPlayer(screen)

	g.hud.Draw(screen)

	if g.hitboxDebug {
		g.drawHitboxes(screen)
	}

	g.overlay.Draw(screen)

	if g.state == GameOverState {
		g.gameOver.Draw(screen)
	}
}

func (g *Game) drawPlayer(screen *ebiten.Image) {
	if g.player.Current == nil || g.player.Current.CurrentFrame() == nil {
		return
	}
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(-120.0/2, -80.0)
	if g.player.Facing < 0 {
		op.GeoM.Scale(-1, 1)
	}
	op.GeoM.Scale(float64(g.cfg.RenderScale), float64(g.cfg.RenderScale))
	op.GeoM.Translate(g.player.X, g.player.Y)
	op.Filter = ebiten.FilterNearest
	screen.DrawImage(g.player.Current.CurrentFrame(), op)
}

func (g *Game) drawEnemy(screen *ebiten.Image, e *enemy.Enemy) {
	if e.Current == nil || e.Current.CurrentFrame() == nil {
		return
	}
	// Orc sprite has ~45px transparent padding at frame bottom (visible char
	// occupies roughly middle of 100px frame); anchor the frame so visible
	// feet, not frame bottom, align with e.Y.
	const orcVisibleFootPadding = 45
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(-100.0/2, -100.0+orcVisibleFootPadding)
	if e.Facing < 0 {
		op.GeoM.Scale(-1, 1)
	}
	op.GeoM.Scale(float64(g.cfg.RenderScale), float64(g.cfg.RenderScale))
	op.GeoM.Translate(e.X, e.Y)
	op.Filter = ebiten.FilterNearest
	screen.DrawImage(e.Current.CurrentFrame(), op)
}

func (g *Game) drawHitboxes(screen *ebiten.Image) {
	drawBox := func(anchorX, anchorY float64, facing int, box combat.Box, c color.Color) {
		var minX float64
		if facing >= 0 {
			minX = anchorX + float64(box.OffsetX)
		} else {
			minX = anchorX - float64(box.OffsetX) - float64(box.W)
		}
		minY := anchorY + float64(box.OffsetY)
		vector.StrokeRect(screen, float32(minX), float32(minY), float32(box.W), float32(box.H), 2, c, false)
	}

	drawBox(g.player.X, g.player.Y, g.player.Facing, g.player.Boxes["body"], color.RGBA{0, 0xFF, 0, 0xFF})
	for _, h := range g.player.ActiveHits() {
		drawBox(g.player.X, g.player.Y, g.player.Facing, h, color.RGBA{0xFF, 0, 0, 0xFF})
	}

	for _, e := range g.enemies {
		drawBox(e.X, e.Y, e.Facing, e.Boxes["body"], color.RGBA{0, 0xFF, 0, 0xFF})
		for _, h := range e.ActiveHits() {
			drawBox(e.X, e.Y, e.Facing, h, color.RGBA{0xFF, 0, 0, 0xFF})
		}
	}
}
