package m8negativepointer

import "github.com/lucaf/heiry"

type component struct{ heiry.Node }
type leaf struct{ heiry.Node }

func invalid() {
	d := heiry.Define[leaf]()
	object := &leaf{}
	d.ComponentProvider[leaf]("Part", component{})
	_ = object
}
