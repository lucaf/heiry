package chains

import (
	"fmt"
	"testing"
)

// Every definition assembled from shared levels validates.
func TestDefinitionsValidate(t *testing.T) {
	for name, err := range map[string]error{"buttons": buttons.Validate(), "loggedButtons": loggedButtons.Validate(), "panels": panels.Validate(), "labeledButtons": labeledButtons.Validate()} {
		if err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}

// Each object dispatches through the registrations of every level it takes,
// and of the parts its levels use, and the wrapper forwards through them.
func TestDispatchThroughSharedLevels(t *testing.T) {
	b := &button{}
	if err := buttonsInit(b); err != nil {
		t.Fatal(err)
	}
	w := &buttonView{state: b}
	if got := w.Name() + ", " + w.Click() + ", " + w.String(); got != "base, button click, a widget" {
		t.Errorf("button = %q", got)
	}

	l := &logged{}
	if err := loggedButtonsInit(l); err != nil {
		t.Fatal(err)
	}
	if got := l.Node.Virtual[NameOp]().Name() + ", " + l.Node.Virtual[ClickOp]().Click(); got != "logged, button click" {
		t.Errorf("logged button = %q", got)
	}

	p := &panel{}
	if err := panelsInit(p); err != nil {
		t.Fatal(err)
	}
	if got := p.Node.Virtual[NameOp]().Name() + ", " + p.Node.Virtual[ClickOp]().Click() + ", " + p.Node.Virtual[fmt.Stringer]().String(); got != "panel, base click, a frame" {
		t.Errorf("panel = %q", got)
	}

	// The part's component registers at the level that uses it.
	b2 := &labeled{}
	if err := labeledButtonsInit(b2); err != nil {
		t.Fatal(err)
	}
	if got := b2.Node.Virtual[ClickOp]().Click() + ", " + b2.Node.Virtual[fmt.Stringer]().String(); got != "button click, a label" {
		t.Errorf("labeled button = %q", got)
	}
}
