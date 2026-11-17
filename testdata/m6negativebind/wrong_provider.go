package m6negativebind

import "github.com/lucaf/heiry"

type read interface{ Read() int }
type bad struct{ heiry.Node }

func invalidTypedBindingInFluentChain() {
	heiry.Define[bad]().Level[bad, read, interface{}]("").Bind[read]((*bad)(nil), ".")
}
