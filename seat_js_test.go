//go:build js && wasm

package seat

import (
	"errors"
	"syscall/js"
	"testing"
)

// fakeDOM installs as much of a document as the seat uses: Node has none.
const fakeDOM = `
function el(tag) {
  var e = { tagName: tag, children: [], attrs: {}, style: {}, isConnected: false, parent: null, focused: 0 };
  e.appendChild = function (c) { c.parent = e; c.isConnected = true; e.children.push(c); return c; };
  e.remove = function () { if (e.parent) e.parent.children = e.parent.children.filter(function (c) { return c !== e; }); e.isConnected = false; };
  e.setAttribute = function (k, v) { e.attrs[k] = v; };
  e.removeAttribute = function (k) { delete e.attrs[k]; };
  e.querySelector = function () { return null; };
  e.focus = function () { e.focused++; globalThis.document.activeElement = e; };
  return e;
}
var listeners = {};
globalThis.addEventListener = function (t, f) { (listeners[t] = listeners[t] || []).push(f); };
globalThis.removeEventListener = function (t, f) { listeners[t] = (listeners[t] || []).filter(function (g) { return g !== f; }); };
globalThis.__fire = function (t, e) { (listeners[t] || []).forEach(function (f) { f(e); }); };
globalThis.__listeners = function (t) { return (listeners[t] || []).length; };
var body = el("body"); body.isConnected = true;
globalThis.document = { body: body, activeElement: null, createElement: el };
`

func installDOM(t *testing.T) {
	t.Helper()
	js.Global().Get("Function").New(fakeDOM).Invoke()
}

// pane is a screen that counts what was done to it.
type pane struct {
	mounts, closes int
	shown          []bool
	fail           error
	el             js.Value
}

func (p *pane) Mount(el js.Value) error {
	p.mounts++
	p.el = el
	return p.fail
}
func (p *pane) Close()           { p.closes++ }
func (p *pane) Shown(front bool) { p.shown = append(p.shown, front) }

func visible(el js.Value) bool {
	return el.Get("style").Get("visibility").String() == "visible" && el.Get("attrs").Get("inert").IsUndefined()
}

// A screen is mounted the first time it is shown, and only then; leaving it
// hides it and keeps it.
func TestScreensMountOnceAndStay(t *testing.T) {
	installDOM(t)
	s := New(Options{NoKeys: true})
	defer s.Close()
	a, b := &pane{}, &pane{}
	s.Add("a", a)
	s.Add("b", b)
	if a.mounts+b.mounts != 0 {
		t.Fatal("adding a screen mounted it")
	}
	for _, n := range []string{"a", "b", "a", "b"} {
		if err := s.Show(n); err != nil {
			t.Fatal(err)
		}
	}
	if a.mounts != 1 || b.mounts != 1 {
		t.Errorf("mounted a %d times, b %d", a.mounts, b.mounts)
	}
	if visible(a.el) || !visible(b.el) {
		t.Errorf("with b in front a is visible=%v, b visible=%v", visible(a.el), visible(b.el))
	}
	if s.Current() != "b" {
		t.Errorf("current is %q", s.Current())
	}
	if got := len(b.shown); got != 3 || !b.shown[0] || b.shown[1] || !b.shown[2] {
		t.Errorf("b was told %v", b.shown)
	}
}

// The page is a screen too: in front, every screen of the seat's is hidden,
// and its function hears it come and go.
func TestThePageIsAScreen(t *testing.T) {
	installDOM(t)
	s := New(Options{NoKeys: true})
	defer s.Close()
	var told []bool
	c := &pane{}
	s.AddPage("page", func(front bool) { told = append(told, front) })
	s.Add("console", c)
	var switches []string
	s.OnSwitch(func(from, to string) { switches = append(switches, from+">"+to) })
	_ = s.Show("page")    //nolint:errcheck // a known screen
	_ = s.Show("console") //nolint:errcheck // a known screen
	_ = s.Show("page")    //nolint:errcheck // a known screen
	if visible(c.el) {
		t.Error("the console shows over the page")
	}
	if len(told) != 3 || !told[0] || told[1] || !told[2] {
		t.Errorf("the page was told %v", told)
	}
	if want := []string{">page", "page>console", "console>page"}; len(switches) != 3 || switches[0] != want[0] || switches[1] != want[1] || switches[2] != want[2] {
		t.Errorf("switches %v, want %v", switches, want)
	}
}

func TestAScreenThatWillNotMountIsNotShown(t *testing.T) {
	installDOM(t)
	s := New(Options{NoKeys: true})
	defer s.Close()
	s.AddPage("page", nil)
	s.Add("broken", &pane{fail: errors.New("no")})
	_ = s.Show("page") //nolint:errcheck // a known screen
	if err := s.Show("broken"); err == nil || s.Current() != "page" {
		t.Errorf("a failed mount gave %v and left %q in front", err, s.Current())
	}
	if err := s.Show("nosuch"); !errors.Is(err, ErrNoScreen) {
		t.Errorf("an unknown screen gave %v", err)
	}
}

// Ctrl+Alt+2 is the second screen, and nothing else on the page sees it.
func TestCtrlAltDigitSwitches(t *testing.T) {
	installDOM(t)
	s := New(Options{})
	s.AddPage("page", nil)
	s.Add("console", &pane{})
	_ = s.Show("page") //nolint:errcheck // a known screen
	stopped := 0
	ev := map[string]any{
		"ctrlKey": true, "altKey": true, "shiftKey": false, "metaKey": false, "code": "Digit2", "key": "2",
		"preventDefault":  js.FuncOf(func(js.Value, []js.Value) any { return nil }),
		"stopPropagation": js.FuncOf(func(js.Value, []js.Value) any { stopped++; return nil }),
	}
	js.Global().Call("__fire", "keydown", ev)
	if s.Current() != "console" || stopped != 1 {
		t.Errorf("Ctrl+Alt+2 left %q in front, stopped %d times", s.Current(), stopped)
	}
	ev["code"], ev["key"] = "Digit5", "5"
	js.Global().Call("__fire", "keydown", ev)
	if s.Current() != "console" || stopped != 1 {
		t.Errorf("Ctrl+Alt+5, with two screens, went to %q", s.Current())
	}
	s.Close()
	if n := js.Global().Call("__listeners", "keydown").Int(); n != 0 {
		t.Errorf("closed, the seat left %d key listeners", n)
	}
}
