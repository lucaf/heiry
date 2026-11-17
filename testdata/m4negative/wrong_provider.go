package m4negative

import "github.com/lucaf/heiry"

type read interface{ Read() int }
type bad struct{ heiry.Node }

func invalidTypedBinding() {
	heiry.Define[bad]().Level[bad, read, interface{}]("").Bind[read]((*bad)(nil), ".")
}
