package game

import (
	"bufio"
	"encoding/json"
	"fmt"
	"image/color"
	"log"
	"math"
	"math/rand"
	"net"
	"sort"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"claude-pixel/internal/aienv"
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
	ModeTimeOut
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
	TimeoutS      float64
	AIPort        int
	AIBothPort    int
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
	timeout           *hud.TimeOut
	timeoutS          float64
	elapsedS          float64
	kinds             []*enemy.Kind
	anims             map[string]*anim.Animation
	physics           *player.Physics
	staminaTuning     *player.StaminaTuning
	rng               *rand.Rand
	score             *score.Counter
	swallowNextIntent bool
	aiListener        net.Listener
	aiConn            net.Conn
	aiReader          *bufio.Reader
	aiWriter          *bufio.Writer
	aiBothMode        bool
}

type livesProvider struct{ p *player.Player }

func (l livesProvider) Lives() int { return l.p.Lives }

type staminaProvider struct{ pool *stamina.Pool }

func (s staminaProvider) StaminaFraction() float64 { return s.pool.Fraction() }

type scoreProvider struct{ c *score.Counter }

func (s scoreProvider) Score() int { return s.c.Total() }

type timerProvider struct{ g *Game }

func (t timerProvider) RemainingS() float64 {
	r := t.g.timeoutS - t.g.elapsedS
	if r < 0 {
		return 0
	}
	return r
}

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
		timerProvider{g},
		d.Layout, d.Cfg.WindowW, d.Cfg.WindowH,
	)
	g.gameOver = hud.NewGameOver(d.OverTitle, d.OverSubtitle, d.Cfg.WindowW, d.Cfg.WindowH)
	g.pause = hud.NewPause(d.OverTitle, d.OverSubtitle, d.Cfg.WindowW, d.Cfg.WindowH)
	g.timeout = hud.NewTimeOut(d.OverTitle, d.OverSubtitle, d.Cfg.WindowW, d.Cfg.WindowH)
	g.timeoutS = d.TimeoutS
	g.toast = hud.NewToast(d.OverSubtitle, d.Cfg.WindowW)

	aiPort := d.AIPort
	if d.AIBothPort > 0 {
		aiPort = d.AIBothPort
		g.aiBothMode = true
	}
	if aiPort > 0 {
		ln, err := net.Listen("tcp", fmt.Sprintf(":%d", aiPort))
		if err != nil {
			log.Fatalf("AI: listen on port %d: %v", aiPort, err)
		}
		g.aiListener = ln
		mode := "player"
		if g.aiBothMode {
			mode = "player+orc"
		}
		log.Printf("AI (%s): listening on :%d — waiting for agent to connect...", mode, aiPort)
		conn, err := ln.Accept()
		if err != nil {
			log.Fatalf("AI: accept: %v", err)
		}
		g.aiConn = conn
		g.aiReader = bufio.NewReader(conn)
		g.aiWriter = bufio.NewWriter(conn)
		log.Printf("AI: agent connected")
	}

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

// AI socket protocol messages (mirror cmd/train/protocol.go).
type aiObsMsg struct {
	Type   string                 `json:"type"`
	Obs    []float64              `json:"obs"`
	Reward float64                `json:"reward"`
	Done   bool                   `json:"done"`
	Info   map[string]interface{} `json:"info,omitempty"`
}

type aiActionMsg struct {
	Type   string `json:"type"`
	Action int    `json:"action"`
}

type aiBothObsMsg struct {
	Type       string         `json:"type"`
	PlayerObs  []float64      `json:"player_obs"`
	OrcObs     [][]float64    `json:"orc_obs"`
	OrcRewards []float64      `json:"orc_rewards"`
	OrcDones   []bool         `json:"orc_dones"`
	Done       bool           `json:"done"`
	Info       map[string]any `json:"info,omitempty"`
}

type aiBothActionMsg struct {
	Type         string `json:"type"`
	PlayerAction int    `json:"player_action"`
	OrcActions   []int  `json:"orc_actions"`
}

func (g *Game) aiReadMsg() (string, int) {
	line, err := g.aiReader.ReadBytes('\n')
	if err != nil {
		log.Printf("AI: read error: %v", err)
		return "", 0
	}
	var msg aiActionMsg
	if err := json.Unmarshal(line, &msg); err != nil {
		log.Printf("AI: unmarshal error: %v", err)
		return "", 0
	}
	return msg.Type, msg.Action
}

func (g *Game) aiSendObs(reward float64, done bool) {
	obs := g.aiObserve()
	msg := aiObsMsg{Type: "obs", Obs: obs[:], Reward: reward, Done: done, Info: map[string]interface{}{
		"score":   g.score.Total(),
		"lives":   g.player.Lives,
		"elapsed": g.elapsedS,
	}}
	data, _ := json.Marshal(msg)
	g.aiWriter.Write(data)
	g.aiWriter.WriteByte('\n')
	g.aiWriter.Flush()
}

func (g *Game) aiObserve() [aienv.ObsSize]float64 {
	enemies := make([]aienv.EnemyState, 0, len(g.enemies))
	for _, e := range g.enemies {
		stateIdx := 0
		attacking := false
		switch e.CurrentState {
		case "run":
			stateIdx = 1
		case "attack":
			stateIdx = 2
			attacking = true
		case "attack2":
			stateIdx = 3
			attacking = true
		case "hurt":
			stateIdx = 4
		case "death":
			stateIdx = 5
		}
		enemies = append(enemies, aienv.EnemyState{
			RelX:      e.X - g.player.X,
			RelY:      e.Y - g.player.Y,
			Lives:     e.Lives,
			MaxLives:  int(e.Kind.Tuning.MaxLives),
			State:     stateIdx,
			Attacking: attacking,
		})
	}

	stateIdx := 0
	switch g.player.FSM.CurrentID() {
	case player.StateRun:
		stateIdx = 1
	case player.StateJump:
		stateIdx = 2
	case player.StateFall:
		stateIdx = 3
	case player.StateAttack:
		stateIdx = 4
	case player.StateAttack2:
		stateIdx = 5
	case player.StateHit:
		stateIdx = 6
	case player.StateDeath:
		stateIdx = 7
	}

	staminaFrac := 0.0
	if g.player.Stamina != nil {
		staminaFrac = g.player.Stamina.Fraction()
	}

	return aienv.Observe(aienv.GameState{
		PlayerX:   g.player.X,
		PlayerY:   g.player.Y,
		PlayerVX:  g.player.VX,
		PlayerVY:  g.player.VY,
		Facing:    g.player.Facing,
		Grounded:  g.player.Grounded,
		Lives:     g.player.Lives,
		MaxLives:  g.combatTuning.SoldierMaxLives,
		Stamina:   staminaFrac,
		StateID:   stateIdx,
		NumStates: 8,
		TimeoutS:  g.timeoutS,
		ElapsedS:  g.elapsedS,
		Enemies:   enemies,
		MaxAlive:  g.spawner.MaxAlive,
		Score:     g.score.Total(),
		MaxSpeed:  g.player.Physics.SprintSpeed,
		MaxFall:   g.player.Physics.MaxFallSpeed,
		WindowW:   float64(g.cfg.WindowW),
		WindowH:   float64(g.cfg.WindowH),
	})
}

func (g *Game) aiBothReadMsg() (string, input.Intent, []int) {
	line, err := g.aiReader.ReadBytes('\n')
	if err != nil {
		log.Printf("AI: read error: %v", err)
		return "", input.Intent{}, nil
	}
	var msg aiBothActionMsg
	if err := json.Unmarshal(line, &msg); err != nil {
		log.Printf("AI: unmarshal error: %v", err)
		return "", input.Intent{}, nil
	}
	return msg.Type, aienv.ToIntent(msg.PlayerAction), msg.OrcActions
}

func (g *Game) aiBothSendObs(done bool) {
	playerObs := g.aiObserve()
	orcObs := make([][]float64, 0, len(g.enemies))
	orcRewards := make([]float64, len(g.enemies))
	orcDones := make([]bool, len(g.enemies))
	for i, e := range g.enemies {
		obs := g.orcObserveEnemy(e)
		orcObs = append(orcObs, obs[:])
		orcDones[i] = e.Dead || e.CurrentState == "death"
	}
	msg := aiBothObsMsg{
		Type: "obs", PlayerObs: playerObs[:],
		OrcObs: orcObs, OrcRewards: orcRewards, OrcDones: orcDones,
		Done: done,
		Info: map[string]any{
			"player_lives": g.player.Lives,
			"orc_count":    len(g.enemies),
			"elapsed":      g.elapsedS,
		},
	}
	data, _ := json.Marshal(msg)
	g.aiWriter.Write(data)
	g.aiWriter.WriteByte('\n')
	g.aiWriter.Flush()
}

func (g *Game) aiBothWaitReset() {
	line, err := g.aiReader.ReadBytes('\n')
	if err != nil {
		log.Printf("AI: read error waiting for reset: %v", err)
		return
	}
	var msg aiBothActionMsg
	json.Unmarshal(line, &msg)
	g.reset()
	g.mode = ModePlaying
	g.aiBothSendObs(false)
}

func (g *Game) applyOrcRLActions(orcActions []int, dt time.Duration) {
	for i, e := range g.enemies {
		if e.Dead || e.CurrentState == "death" || e.CurrentState == "hurt" || e.CurrentState == "fall" {
			e.Tick(dt)
			continue
		}
		if e.CurrentState == "attack" || e.CurrentState == "attack2" {
			e.Tick(dt)
			continue
		}
		action := 0
		if i < len(orcActions) {
			action = orcActions[i]
		}
		ar := aienv.OrcAction(action)
		if ar.Transition != "" && e.CurrentState == "run" {
			e.Transition(ar.Transition)
			continue
		}
		if ar.Flip {
			e.Facing = -e.Facing
			e.VX = 0
			continue
		}
		speed := 120.0
		switch ar.VXMode {
		case aienv.OrcVXStop:
			e.VX = 0
		case aienv.OrcVXToward:
			if g.player.X > e.X {
				e.Facing = 1
			} else {
				e.Facing = -1
			}
			e.VX = float64(e.Facing) * speed
		case aienv.OrcVXAway:
			if g.player.X > e.X {
				e.Facing = -1
			} else {
				e.Facing = 1
			}
			e.VX = float64(e.Facing) * speed
		}
		if e.Current != nil {
			e.Current.Update(dt)
		}
	}
	for _, e := range g.enemies {
		if e.Lives <= 0 && e.CurrentState != "death" {
			e.Transition("death")
		}
		if e.OnHitPending {
			e.OnHitPending = false
			e.Transition("hurt")
		}
	}
}

func (g *Game) orcObserveEnemy(e *enemy.Enemy) [aienv.OrcObsSize]float64 {
	orcStateIdx := 0
	switch e.CurrentState {
	case "run":
		orcStateIdx = 1
	case "attack":
		orcStateIdx = 2
	case "attack2":
		orcStateIdx = 3
	case "hurt":
		orcStateIdx = 4
	case "death":
		orcStateIdx = 5
	}
	playerStateIdx := 0
	switch g.player.FSM.CurrentID() {
	case player.StateRun:
		playerStateIdx = 1
	case player.StateJump:
		playerStateIdx = 2
	case player.StateFall:
		playerStateIdx = 3
	case player.StateAttack:
		playerStateIdx = 4
	case player.StateAttack2:
		playerStateIdx = 5
	case player.StateHit:
		playerStateIdx = 6
	case player.StateDeath:
		playerStateIdx = 7
	}
	playerAttacking := g.player.CurrentAnim == "soldier_attack" || g.player.CurrentAnim == "soldier_attack2"
	return aienv.OrcObserve(aienv.OrcGameState{
		OrcX: e.X, OrcY: e.Y, OrcVX: e.VX, OrcVY: e.VY,
		OrcFacing: e.Facing, OrcGrounded: e.Grounded,
		OrcLives: e.Lives, OrcMaxLives: int(e.Kind.Tuning.MaxLives),
		OrcStateID: orcStateIdx, OrcNumStates: 6,
		PlayerX: g.player.X, PlayerY: g.player.Y,
		PlayerLives: g.player.Lives, PlayerMaxLives: g.combatTuning.SoldierMaxLives,
		PlayerStateID: playerStateIdx, PlayerNumStates: 8,
		PlayerAttacking: playerAttacking, PlayerFacing: g.player.Facing,
		OrcMaxSpeed: 120, OrcMaxFall: g.player.Physics.MaxFallSpeed,
		TimeoutS: g.timeoutS, ElapsedS: g.elapsedS,
		WindowW: float64(g.cfg.WindowW), WindowH: float64(g.cfg.WindowH),
	})
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

	if g.mode == ModeGameOver || g.mode == ModeTimeOut {
		if g.aiConn != nil && g.aiBothMode {
			g.aiBothWaitReset()
			return nil
		}
		if g.aiConn != nil {
			msgType, _ := g.aiReadMsg()
			if msgType == "reset" {
				g.reset()
				g.mode = ModePlaying
				g.aiSendObs(0, false)
			}
			return nil
		}
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

	var intent input.Intent
	var orcActions []int
	if g.aiConn != nil && g.aiBothMode {
		var msgType string
		msgType, intent, orcActions = g.aiBothReadMsg()
		if msgType == "reset" {
			g.reset()
			g.mode = ModePlaying
			g.aiBothSendObs(false)
			return nil
		}
	} else if g.aiConn != nil {
		msgType, action := g.aiReadMsg()
		if msgType == "reset" {
			g.reset()
			g.aiSendObs(0, false)
			return nil
		}
		intent = aienv.ToIntent(action)
	} else {
		intent = input.Poll()
	}
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
	if g.aiBothMode && orcActions != nil {
		g.applyOrcRLActions(orcActions, dt)
	} else {
		for _, e := range g.enemies {
			e.Tick(dt)
		}
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

	g.elapsedS += dt.Seconds()
	if g.timeoutS > 0 && g.elapsedS >= g.timeoutS {
		g.mode = ModeTimeOut
	}

	if g.player.FSM.CurrentID() == player.StateDeath && g.player.Current != nil && g.player.Current.Done() {
		g.mode = ModeGameOver
	}

	if g.aiConn != nil && g.aiBothMode {
		done := g.mode == ModeGameOver || g.mode == ModeTimeOut
		g.aiBothSendObs(done)
	} else if g.aiConn != nil {
		done := g.mode == ModeGameOver || g.mode == ModeTimeOut
		reward := 0.0
		if done {
			reward = -50.0
		}
		g.aiSendObs(reward, done)
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
	g.elapsedS = 0

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
	if g.mode == ModeTimeOut {
		g.timeout.Draw(screen, g.score.Total())
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
