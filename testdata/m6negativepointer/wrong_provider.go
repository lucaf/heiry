package m6negativepointer

import "github.com/lucaf/heiry"

type read interface{ Read() int }
type root struct{ heiry.Node }

func invalidGroupedPointer() {
	heiry.Define[root]().Level[root, read, interface{}]("").BindGroup(root{}, ".", heiry.Of[read]())
}
