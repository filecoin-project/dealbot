package tasks

import (
	"context"
	"fmt"

	ipld "github.com/ipld/go-ipld-prime"
)

type localLink struct {
	ipld.Node
}

var _ ipld.Link = (*localLink)(nil)

func (l *localLink) LinkBuilder() ipld.LinkBuilder {
	return nil
}

func (l *localLink) Load(_ context.Context, _ ipld.LinkContext, na ipld.NodeAssembler, _ ipld.Loader) error {
	return na.AssignNode(l.Node)
}

func (l *localLink) String() string {
	return fmt.Sprintf("&%s", l.Node)
}

func (l *localLink) Representation() ipld.Node {
	return l.Node
}
