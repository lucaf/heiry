package chains

import "github.com/lucaf/heiry"

type labeler struct{ heiry.Node }

func (*labeler) String() string { return "a label" }

// labelingPart is alone in its file, and its line uses an import, so the
// file's output holds the part's freshness assertion and imports nothing.
//
//heiry:spec
const labelingPart = `
import fmt "fmt"
part labeling
	component label labeler fmt.Stringer
`
