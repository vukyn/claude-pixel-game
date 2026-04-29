package game

import (
	"fmt"
	"image/color"
	"log"
	"math"
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
	"claude-pixel/internal/score"
	"claude-pixel/internal/spawner"
	"claude-pixel/internal/stamina"
	"claude-pixel/internal/world"
)

type Mode int

const (
	ModePlaying Mode = iota
	ModePaused
	ModeGameOver
)

type Deps struct {
	Cfg           *config.Config
	Anims         map[string]*anim.Animation
	Physics       *player.Physics
	StaminaTuning *player.StaminaTuning
	DebugCfg      *debug.Config
	SoldierBoxes  map[string]combat.Box
	CombatTuning  *combat.Tuning
	EnemyKinds    []*enemy.Kind
	SpawnTuning   *enemy.SpawnTuning
	HeartAnim     *anim.Animation
	StaminaAnim   *anim.Animation
	HUDFace       *text.GoTextFace
	OverTitle     *text.GoTextFace
	OverSubtitle  *text.GoTextFace
	Layout        hud.Layout
}

type Game struct {
	cfg               *config.Config
	world             *world.World
	player            *player.Player
	enemies           []*enemy.Enemy
	spawner           *spawner.Spawner
	overlay           *debug.Overlay
	hud               *hud.HUD
	gameOver          *hud.GameOver
	pause             *hud.Pause
	toast             *hud.Toast
	mode              Mode
	hitboxDebug       bool
	lastIntent        input.Intent
	combatTuning      *combat.Tuning
	kinds             []*enemy.Kind
	anims             map[string]*anim.Animation
	physics           *player.Physics
	staminaTuning     *player.StaminaTuning
	rng               *rand.Rand
	score             *score.Counter
	swallowNextIntent bool
}

type livesProvider struct{ p *player.Player }

func (l livesProvider) Lives() int { return l.p.Lives }

type staminaProvider struct{ pool *stamina.Pool }

func (s staminaProvider) StaminaFraction() float64 { return s.pool.Fraction() }

type scoreProvider struct{ c *score.Counter }

func (s scoreProvider) Score() int { return s.c.Total() }

func New(d Deps) *Game {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	w := world.New(d.Cfg, d.Physics.Gravity)

	pool := stamina.NewPool(d.StaminaTuning.Max, d.StaminaTuning.DrainPerSec, d.StaminaTuning.RegenPerSec)

	p := player.New(player.Config{
		StartX:     float64(d.Cfg.WindowW) / 2,
		StartY:     w.GroundY,
		Physics:    d.Physics,
		Anims:      d.Anims,
		Boxes:      d.SoldierBoxes,
		StartLives: d.CombatTuning.SoldierMaxLives,
		Stamina:    pool,
	})
	p.Grounded = true

	sc := &score.Counter{}

	g := &Game{
		cfg:           d.Cfg,
		world:         w,
		player:        p,
		combatTuning:  d.CombatTuning,
		kinds:         d.EnemyKinds,
		anims:         d.Anims,
		physics:       d.Physics,
		staminaTuning: d.StaminaTuning,
		rng:           rng,
		mode:          ModePlaying,
		score:         sc,
	}
	g.overlay = debug.NewOverlay(d.DebugCfg, g)

	kindFactories := make([]spawner.KindFactory, 0, len(d.EnemyKinds))
	for _, k := range d.EnemyKinds {
		k := k
		halfW := float64(k.Boxes["body"].W) / 2
		spriteH := float64(k.FrameH * d.Cfg.RenderScale)
		kindFactories = append(kindFactories, spawner.KindFactory{
			Name:   k.Name,
			Weight: 1,
			NewEnemy: func(x, _ float64) *enemy.Enemy {
				if x < halfW {
					x = halfW
				}
				if maxX := float64(d.Cfg.WindowW) - halfW; x > maxX {
					x = maxX
				}
				return enemy.New(enemy.Config{
					StartX: x, StartY: -spriteH,
					Physics: d.Physics,
					Kind:    k,
					RNG:     rng,
				})
			},
		})
	}

	g.spawner = spawner.New(spawner.Config{
		MinIntervalS: d.SpawnTuning.MinS,
		MaxIntervalS: d.SpawnTuning.MaxS,
		MaxAlive:     d.SpawnTuning.MaxAlive,
		SpawnXMin:    0,
		SpawnXMax:    float64(d.Cfg.WindowW),
		RNG:          rng,
		Kinds:        kindFactories,
	})

	g.hud = hud.NewHUD(
		d.HeartAnim, d.StaminaAnim, d.HUDFace,
		livesProvider{p}, staminaProvider{pool}, scoreProvider{sc},
		d.Layout, d.Cfg.WindowW, d.Cfg.WindowH,
	)
	g.gameOver = hud.NewGameOver(d.OverTitle, d.OverSubtitle, d.Cfg.WindowW, d.Cfg.WindowH)
	g.pause = hud.NewPause(d.OverTitle, d.OverSubtitle, d.Cfg.WindowW, d.Cfg.WindowH)
	g.toast = hud.NewToast(d.OverSubtitle, d.Cfg.WindowW)

	return g
}

func (g *Game) Player() *player.Player { return g.player }
func (g *Game) Intent() *input.Intent  { return &g.lastIntent }
func (g *Game) EngineFPS() float64     { return ebiten.ActualFPS() }
func (g *Game) EngineTPS() float64     { return ebiten.ActualTPS() }
func (g *Game) EnemyCount() int        { return len(g.enemies) }
func (g *Game) NextSpawnS() float64    { return g.spawner.NextSpawnS() }

func (g *Game) NearestEnemyState() string {
	e := g.nearestEnemy()
	if e == nil {
		return "(none)"
	}
	return e.CurrentState
}

func (g *Game) NearestEnemyBranch() string {
	e := g.nearestEnemy()
	if e == nil {
		return ""
	}
	return e.BranchTag
}

func (g *Game) nearestEnemy() *enemy.Enemy {
	if len(g.enemies) == 0 {
		return nil
	}
	px := g.player.X
	var best *enemy.Enemy
	bestD := math.MaxFloat64
	for _, e := range g.enemies {
		d := e.X - px
		if d < 0 {
			d = -d
		}
		if d < bestD {
			bestD = d
			best = e
		}
	}
	return best
}

func (g *Game) Layout(outerW, outerH int) (int, int) { return g.cfg.WindowW, g.cfg.WindowH }

func (g *Game) Update() error {
	if inpututil.IsKeyJustPressed(ebiten.KeyF3) {
		g.overlay.Toggle()
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyF4) {
		g.hitboxDebug = !g.hitboxDebug
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyF5) {
		var firstErr error
		failed := ""
		succeeded := 0
		for _, k := range g.kinds {
			if err := enemy.ReloadBehavior(k, g.anims); err != nil {
				log.Printf("behavior reload failed for %q: %v", k.Name, err)
				if firstErr == nil {
					firstErr = err
					failed = k.Name
				}
			} else {
				succeeded++
			}
		}
		if firstErr != nil {
			msg := fmt.Sprintf("Reload failed (%s): %v", failed, firstErr)
			if len(msg) > 80 {
				msg = msg[:77] + "..."
			}
			g.toast.Show(msg, 4*time.Second)
		} else {
			g.toast.Show(fmt.Sprintf("Behaviors reloaded (%d)", succeeded), 2500*time.Millisecond)
		}
	}

	if g.mode == ModeGameOver {
		if inpututil.IsKeyJustPressed(ebiten.KeyR) {
			g.reset()
		}
		return nil
	}

	if g.mode == ModePaused {
		if len(inpututil.AppendJustPressedKeys(nil)) > 0 {
			g.mode = ModePlaying
			g.swallowNextIntent = true
		}
		return nil
	}

	intent := input.Poll()
	if intent.PauseEdge {
		g.mode = ModePaused
		return nil
	}
	if g.swallowNextIntent {
		intent.JumpPressed = false
		intent.AttackPressed = false
		intent.Attack2Pressed = false
		intent.PauseEdge = false
		g.swallowNextIntent = false
	}
	g.lastIntent = intent
	dt := time.Second / 60

	if g.player.Stamina != nil {
		g.player.Stamina.Update(dt, g.player.IsSprinting(intent))
	}

	g.player.FSM.Handle(g.player, g.lastIntent, dt)
	for _, e := range g.enemies {
		e.Tick(dt)
	}

	g.player.ApplyPhysics(g.world, dt)
	for _, e := range g.enemies {
		e.ApplyPhysics(g.world, dt)
	}

	soldierBodyHalfW := float64(g.player.Boxes["body"].W) / 2
	g.player.X = world.Clamp(g.player.X, soldierBodyHalfW, float64(g.cfg.WindowW)-soldierBodyHalfW)

	for _, e := range g.enemies {
		bodyHalfW := float64(e.Kind.Boxes["body"].W) / 2
		leftLimit := bodyHalfW
		rightLimit := float64(g.cfg.WindowW) - bodyHalfW
		clamped := world.Clamp(e.X, leftLimit, rightLimit)
		if clamped != e.X && e.CurrentState == "run" {
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
	g.hud.Update(dt)

	if spawned := g.spawner.Tick(dt, len(g.enemies)); spawned != nil {
		g.enemies = append(g.enemies, spawned)
	}

	g.dispatchSoldierHits()
	g.dispatchOrcHits()

	alive := g.enemies[:0]
	for _, e := range g.enemies {
		if e.Dead {
			g.score.Add(e.Kind.Tuning.Points)
			continue
		}
		alive = append(alive, e)
	}
	g.enemies = alive

	if g.player.FSM.CurrentID() == player.StateDeath && g.player.Current != nil && g.player.Current.Done() {
		g.mode = ModeGameOver
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

	pool := stamina.NewPool(g.staminaTuning.Max, g.staminaTuning.DrainPerSec, g.staminaTuning.RegenPerSec)
	g.player = player.New(player.Config{
		StartX:     float64(g.cfg.WindowW) / 2,
		StartY:     g.world.GroundY,
		Physics:    g.physics,
		Anims:      oldAnims,
		Boxes:      oldBoxes,
		StartLives: g.combatTuning.SoldierMaxLives,
		Stamina:    pool,
	})
	g.player.Grounded = true

	g.score.Reset()

	g.hud.Lives = livesProvider{g.player}
	g.hud.Stamina = staminaProvider{pool}
	g.mode = ModePlaying
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

	if g.mode == ModePaused {
		g.pause.Draw(screen)
	}
	if g.mode == ModeGameOver {
		g.gameOver.Draw(screen)
	}

	g.toast.Draw(screen)
}

func (g *Game) drawPlayer(screen *ebiten.Image) {
	if g.player.Current == nil || g.player.Current.CurrentFrame() == nil {
		return
	}
	pad := g.combatTuning.SoldierFootPadding
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(-120.0/2, -80.0+float64(pad))
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
	pad := e.Kind.Tuning.FootPadding
	fw := float64(e.Kind.FrameW)
	fh := float64(e.Kind.FrameH)
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(-fw/2, -fh+float64(pad))
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
		drawBox(e.X, e.Y, e.Facing, e.Kind.Boxes["body"], color.RGBA{0, 0xFF, 0, 0xFF})
		for _, h := range e.ActiveHits() {
			drawBox(e.X, e.Y, e.Facing, h, color.RGBA{0xFF, 0, 0, 0xFF})
		}
	}
}
