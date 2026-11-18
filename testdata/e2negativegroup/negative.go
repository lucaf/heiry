package e2negativegroup

import "github.com/lucaf/heiry"

type read interface{ Read() int }

type leaf struct{ heiry.Node }

func (l *leaf) Read() int { return 0 }

func invalid() {
	d := heiry.Define[leaf]()
	d.Level[leaf, read, interface{}]("").BindNodeGroup(leaf{}, heiry.Of[read]())
}
