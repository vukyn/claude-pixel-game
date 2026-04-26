package http_test

import (
	"bytes"
	"encoding/json"
	"errors"
	httpstd "net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"

	"claude-pixel/internal/behavior"
	"claude-pixel/internal/editor/http"
	"claude-pixel/internal/editor/port"
	"claude-pixel/internal/editor/service"
)

type stubBehaviorStore struct {
	listFn func() ([]port.BehaviorRef, error)
	getFn  func(string) ([]byte, error)
	putFn  func(string, []byte) error
}

func (s stubBehaviorStore) List() ([]port.BehaviorRef, error) { return s.listFn() }
func (s stubBehaviorStore) Get(k string) ([]byte, error)      { return s.getFn(k) }
func (s stubBehaviorStore) Put(k string, raw []byte) error    { return s.putFn(k, raw) }

type stubTuningStore struct {
	listFn   func(string) ([]port.TuningRow, error)
	updateFn func(string, float64) (float64, error)
}

func (s stubTuningStore) List(p string) ([]port.TuningRow, error)     { return s.listFn(p) }
func (s stubTuningStore) Update(k string, v float64) (float64, error) { return s.updateFn(k, v) }

type stubRegistry struct{ a, c []behavior.ActionMeta }

func (s stubRegistry) Actions() []behavior.ActionMeta    { return s.a }
func (s stubRegistry) Conditions() []behavior.ActionMeta { return s.c }

const validOrc = `{"kind":"orc","states":[{"id":"idle","anim":"idle","decision":false,"exit_on":"anim_done","next":"idle"}]}`

func newApp(b port.BehaviorStore, t port.TuningStore, r port.RegistryStore) *fiber.App {
	app := fiber.New()
	http.Register(app, http.Deps{
		Behavior: service.NewBehavior(b),
		Tuning:   service.NewTuning(t),
		Registry: service.NewRegistry(r),
	})
	return app
}

func do(t *testing.T, app *fiber.App, method, path string, body []byte) (int, []byte) {
	t.Helper()
	var r *httpstd.Request
	if body != nil {
		r = httptest.NewRequest(method, path, bytes.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
	} else {
		r = httptest.NewRequest(method, path, nil)
	}
	resp, err := app.Test(r, -1)
	if err != nil {
		t.Fatalf("Test: %v", err)
	}
	buf := new(bytes.Buffer)
	_, _ = buf.ReadFrom(resp.Body)
	return resp.StatusCode, buf.Bytes()
}

func TestGetBehaviors(t *testing.T) {
	app := newApp(stubBehaviorStore{listFn: func() ([]port.BehaviorRef, error) {
		return []port.BehaviorRef{{Kind: "orc", Path: "/x/orc.json", StateCount: 6}}, nil
	}}, nil, nil)
	code, body := do(t, app, "GET", "/api/behaviors", nil)
	if code != 200 {
		t.Fatalf("status %d body %s", code, body)
	}
	if !strings.Contains(string(body), `"kind":"orc"`) {
		t.Fatalf("body: %s", body)
	}
}

func TestGetBehaviorByKind(t *testing.T) {
	app := newApp(stubBehaviorStore{getFn: func(k string) ([]byte, error) {
		if k != "orc" {
			t.Fatalf("kind: %q", k)
		}
		return []byte(validOrc), nil
	}}, nil, nil)
	code, body := do(t, app, "GET", "/api/behaviors/orc", nil)
	if code != 200 || string(body) != validOrc {
		t.Fatalf("status %d body %s", code, body)
	}
}

func TestGetBehaviorNotFound(t *testing.T) {
	app := newApp(stubBehaviorStore{getFn: func(string) ([]byte, error) {
		return nil, errors.New("not found")
	}}, nil, nil)
	code, _ := do(t, app, "GET", "/api/behaviors/ghost", nil)
	if code != 404 {
		t.Fatalf("status %d", code)
	}
}

func TestPutBehaviorValid(t *testing.T) {
	called := false
	app := newApp(stubBehaviorStore{putFn: func(k string, raw []byte) error {
		called = true
		return nil
	}}, nil, nil)
	code, _ := do(t, app, "PUT", "/api/behaviors/orc", []byte(validOrc))
	if code != 200 {
		t.Fatalf("status %d", code)
	}
	if !called {
		t.Fatal("Put not called")
	}
}

func TestPutBehaviorInvalid(t *testing.T) {
	app := newApp(stubBehaviorStore{}, nil, nil)
	bad := `{"kind":"orc","states":[]}`
	code, body := do(t, app, "PUT", "/api/behaviors/orc", []byte(bad))
	if code != 400 {
		t.Fatalf("status %d body %s", code, body)
	}
	var v service.ValidationResult
	_ = json.Unmarshal(body, &v)
	if v.Valid || len(v.Errors) == 0 {
		t.Fatalf("body: %+v", v)
	}
}

func TestValidateBehavior(t *testing.T) {
	app := newApp(stubBehaviorStore{}, nil, nil)
	code, body := do(t, app, "POST", "/api/behaviors/orc/validate", []byte(validOrc))
	if code != 200 {
		t.Fatalf("status %d body %s", code, body)
	}
	if !strings.Contains(string(body), `"valid":true`) {
		t.Fatalf("body %s", body)
	}
}

func TestGetTuning(t *testing.T) {
	app := newApp(nil, stubTuningStore{listFn: func(p string) ([]port.TuningRow, error) {
		return []port.TuningRow{{Key: "orc_max_lives", Value: 2, Min: 1, Max: 10, Unit: "—"}}, nil
	}}, nil)
	code, body := do(t, app, "GET", "/api/tuning?prefix=orc", nil)
	if code != 200 || !strings.Contains(string(body), "orc_max_lives") {
		t.Fatalf("status %d body %s", code, body)
	}
}

func TestPutTuning(t *testing.T) {
	app := newApp(nil, stubTuningStore{updateFn: func(k string, v float64) (float64, error) {
		if k != "orc_max_lives" || v != 5 {
			t.Fatalf("args: %s %v", k, v)
		}
		return 2, nil
	}}, nil)
	code, body := do(t, app, "PUT", "/api/tuning/orc_max_lives", []byte(`{"value":5}`))
	if code != 200 || !strings.Contains(string(body), `"old":2`) {
		t.Fatalf("status %d body %s", code, body)
	}
}

func TestPutTuningOutOfRange(t *testing.T) {
	app := newApp(nil, stubTuningStore{updateFn: func(string, float64) (float64, error) {
		return 0, errors.New("value out of range: 999 not in [1, 10] —")
	}}, nil)
	code, _ := do(t, app, "PUT", "/api/tuning/orc_max_lives", []byte(`{"value":999}`))
	if code != 400 {
		t.Fatalf("status %d", code)
	}
}

func TestGetRegistryActions(t *testing.T) {
	app := newApp(nil, nil, stubRegistry{a: []behavior.ActionMeta{{Name: "goto"}}})
	code, body := do(t, app, "GET", "/api/registry/actions", nil)
	if code != 200 || !strings.Contains(string(body), "goto") {
		t.Fatalf("body %s", body)
	}
}

func TestGetRegistryConditions(t *testing.T) {
	app := newApp(nil, nil, stubRegistry{c: []behavior.ActionMeta{{Name: "grounded"}}})
	code, body := do(t, app, "GET", "/api/registry/conditions", nil)
	if code != 200 || !strings.Contains(string(body), "grounded") {
		t.Fatalf("body %s", body)
	}
}
