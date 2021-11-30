package tasks

import (
	"fmt"

	ipld "github.com/ipld/go-ipld-prime"
)

type localLink struct {
	ipld.Node
}

var _ ipld.Link = (*localLink)(nil)

func (l *localLink) String() string {
	return fmt.Sprintf("&%s", l.Node)
}

func (l *localLink) Binary() string {
	return fmt.Sprintf("&%s", l.Node)
}

func (l *localLink) Representation() ipld.Node {
	return l.Node
}

func (l *localLink) Prototype() ipld.LinkPrototype {
	return nil
}
