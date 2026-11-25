package chains

import "github.com/lucaf/heiry"

type logged struct {
	button
	heiry.Node
}

func (*logged) Name() string { return "logged" }

//heiry:spec
const loggedSpec = `
hierarchy loggedButtons logged from=buttons
level logged
	node NameOp
`
