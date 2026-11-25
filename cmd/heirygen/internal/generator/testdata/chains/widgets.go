// Package chains is a heirygen test fixture for specifications that share
// levels: a chain holds the root level and another chain extends it, and
// hierarchies take their levels with from=, one of them from another
// hierarchy. A taken line keeps its own constant's import, which the outputs
// that take it carry.
package chains

//go:generate go run github.com/lucaf/heiry/cmd/heirygen

import "github.com/lucaf/heiry"

type (
	Public interface {
		Name() string
		Click() string
		String() string
	}
	Private interface{}
)

type base struct{ heiry.Node }

func (*base) Name() string   { return "base" }
func (*base) Click() string  { return "base click" }
func (*base) String() string { return "a widget" }

//heiry:spec
const opsSpec = `
import fmt "fmt"
operations Public Private suffix=Op
	existing String fmt.Stringer
`

//heiry:spec
const widgetsChain = `
import fmt "fmt"
chain widgets
level base public=Public private=Private
	node NameOp ClickOp fmt.Stringer
`
