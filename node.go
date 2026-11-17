package heiry

// Node is the scope holder embedded directly in each inheritance-level
// receiver. An initialized receiver must not be copied.
type Node struct {
	noCopy noCopy
	scope  Scope
}

// Scope returns this receiver's fixed dispatch position.
func (n *Node) Scope() Scope { return n.scope }

// noCopy makes ordinary value copies of initialized nodes visible to go vet.
type noCopy struct{}

func (*noCopy) Lock()   {}
func (*noCopy) Unlock() {}
