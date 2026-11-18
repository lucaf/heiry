package e2negativebindnode

import "github.com/lucaf/heiry"

type read interface{ Read() int }

type leaf struct{ heiry.Node }

func invalid() {
	d := heiry.Define[leaf]()
	d.Level[leaf, read, interface{}]("").BindNode[read]((*leaf)(nil))
}
