package chains

import "github.com/lucaf/heiry"

type frame struct {
	base
	heiry.Node
}

func (*frame) String() string { return "a frame" }

// framedChain is alone in its file, so the file's output holds the chain's
// freshness assertion and nothing for the lines it takes with from=.
//
//heiry:spec
const framedChain = `
import fmt "fmt"
chain framed from=widgets
level frame
	node fmt.Stringer
`
