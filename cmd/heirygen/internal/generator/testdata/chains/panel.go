package chains

import "github.com/lucaf/heiry"

type panel struct {
	frame
	heiry.Node
}

func (*panel) Name() string { return "panel" }

//heiry:spec
const panelSpec = `
hierarchy panels panel from=framed
level panel
	node NameOp
`
