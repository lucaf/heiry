package chains

import "github.com/lucaf/heiry"

type button struct {
	base
	heiry.Node
}

func (*button) Click() string { return "button click" }

//heiry:spec
const buttonSpec = `
hierarchy buttons button from=widgets
level button
	node ClickOp
wrapper buttonView
`
