package m6negativetoken

import "github.com/lucaf/heiry"

type read interface{ Read() int }
type root struct{ heiry.Node }

func invalidGroupedToken() {
	heiry.Define[root]().Level[root, read, interface{}]("").BindGroup((*root)(nil), ".", 7)
}
