package stage5vet

import "github.com/lucaf/heiry"

type containingNode struct{ heiry.Node }

func copiedNode() {
	first := containingNode{}
	second := first
	_ = second
}
