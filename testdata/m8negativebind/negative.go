package m8negativebind

import "github.com/lucaf/heiry"

type read interface{ Read() int }
type secretProvider struct{ heiry.Node }
type leaf struct {
	heiry.Node
	Part secretProvider
}

func invalid() {
	d := heiry.Define[leaf]()
	d.Level[leaf, read, interface{}]("").Bind[read]((*secretProvider)(nil), "Part")
}
