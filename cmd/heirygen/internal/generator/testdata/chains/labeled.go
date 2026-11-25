package chains

import "github.com/lucaf/heiry"

type labeled struct {
	button
	heiry.Node
	label labeler
}

//heiry:spec
const labeledSpec = `
hierarchy labeledButtons labeled from=buttons
level labeled
	use labeling
`
