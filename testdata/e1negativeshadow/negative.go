package e1negativeshadow

import "github.com/lucaf/heiry"

type read interface{ Read() int }

type tree struct {
	heiry.Node
	Parent *tree
}

func invalid(t *tree) {
	_ = t.Parent[read]()
}
