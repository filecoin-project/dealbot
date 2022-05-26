package graphql

import (
	"context"
	"fmt"

	"github.com/filecoin-project/dealbot/tasks"
	"github.com/graphql-go/graphql"
	ipld "github.com/ipld/go-ipld-prime"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
)

type nodeLoader func(ctx context.Context, cid cidlink.Link, builder ipld.NodeBuilder) (ipld.Node, error)

const nodeLoaderCtxKey = "NodeLoader"

var errWrongKindRes = fmt.Errorf("wrong kind of resource")

//var errNotNode = fmt.Errorf("Not IPLD Node")
var errInvalidLoader = fmt.Errorf("Invalid Loader Provided")
var errInvalidLink = fmt.Errorf("Invalid link")
var errUnexpectedType = "Unexpected type %T. expected %s"

func AuthenticatedRecord__Record__resolve(p graphql.ResolveParams) (interface{}, error) {
	ts, ok := p.Source.(*tasks.AuthenticatedRecord)
	if !ok {
		return nil, fmt.Errorf(errUnexpectedType, p.Source, "tasks.AuthenticatedRecord")
	}

	targetCid := ts.Record

	var node ipld.Node

	if cl, ok := targetCid.(cidlink.Link); ok {
		v := p.Context.Value(nodeLoaderCtxKey)
		if v == nil {
			return cl.Cid, nil
		}
		loader, ok := v.(func(context.Context, cidlink.Link, ipld.NodeBuilder) (ipld.Node, error))
		if !ok {
			return nil, errInvalidLoader
		}

		builder := tasks.FinishedTaskPrototype.NewBuilder()
		n, err := loader(p.Context, cl, builder)
		if err != nil {
			return nil, err
		}
		node = n
	} else {
		return nil, errInvalidLink
	}

	return node, nil

}
func AuthenticatedRecord__Signature__resolve(p graphql.ResolveParams) (interface{}, error) {
	ts, ok := p.Source.(*tasks.AuthenticatedRecord)
	if !ok {
		return nil, fmt.Errorf(errUnexpectedType, p.Source, "tasks.AuthenticatedRecord")
	}

	return string(ts.Signature), nil

}

var AuthenticatedRecord__type = graphql.NewObject(graphql.ObjectConfig{
	Name: "AuthenticatedRecord",
	Fields: graphql.Fields{
		"Record": &graphql.Field{

			Type: graphql.NewNonNull(FinishedTask__type),

			Resolve: AuthenticatedRecord__Record__resolve,
		},
		"Signature": &graphql.Field{

			Type: graphql.NewNonNull(graphql.String),

			Resolve: AuthenticatedRecord__Signature__resolve,
		},
	},
})

func FinishedTask__Status__resolve(p graphql.ResolveParams) (interface{}, error) {
	ts, ok := p.Source.(*tasks.FinishedTask)
	if !ok {
		return nil, fmt.Errorf(errUnexpectedType, p.Source, "tasks.FinishedTask")
	}

	return int64(ts.Status), nil

}
func FinishedTask__StartedAt__resolve(p graphql.ResolveParams) (interface{}, error) {
	ts, ok := p.Source.(*tasks.FinishedTask)
	if !ok {
		return nil, fmt.Errorf(errUnexpectedType, p.Source, "tasks.FinishedTask")
	}

	return int64(ts.StartedAt), nil

}
func FinishedTask__ErrorMessage__resolve(p graphql.ResolveParams) (interface{}, error) {
	ts, ok := p.Source.(*tasks.FinishedTask)
	if !ok {
		return nil, fmt.Errorf(errUnexpectedType, p.Source, "tasks.FinishedTask")
	}

	f := ts.ErrorMessage
	if f != nil {

		return *f, nil

	} else {
		return nil, nil
	}

}
func FinishedTask__RetrievalTask__resolve(p graphql.ResolveParams) (interface{}, error) {
	ts, ok := p.Source.(*tasks.FinishedTask)
	if !ok {
		return nil, fmt.Errorf(errUnexpectedType, p.Source, "tasks.FinishedTask")
	}

	f := ts.RetrievalTask
	if f != nil {

		return f, nil

	} else {
		return nil, nil
	}

}
func FinishedTask__StorageTask__resolve(p graphql.ResolveParams) (interface{}, error) {
	ts, ok := p.Source.(*tasks.FinishedTask)
	if !ok {
		return nil, fmt.Errorf(errUnexpectedType, p.Source, "tasks.FinishedTask")
	}

	f := ts.StorageTask
	if f != nil {

		return f, nil

	} else {
		return nil, nil
	}

}
func FinishedTask__DealID__resolve(p graphql.ResolveParams) (interface{}, error) {
	ts, ok := p.Source.(*tasks.FinishedTask)
	if !ok {
		return nil, fmt.Errorf(errUnexpectedType, p.Source, "tasks.FinishedTask")
	}

	return int64(ts.DealID), nil

}
func FinishedTask__MinerMultiAddr__resolve(p graphql.ResolveParams) (interface{}, error) {
	ts, ok := p.Source.(*tasks.FinishedTask)
	if !ok {
		return nil, fmt.Errorf(errUnexpectedType, p.Source, "tasks.FinishedTask")
	}

	return ts.MinerMultiAddr, nil

}
func FinishedTask__ClientApparentAddr__resolve(p graphql.ResolveParams) (interface{}, error) {
	ts, ok := p.Source.(*tasks.FinishedTask)
	if !ok {
		return nil, fmt.Errorf(errUnexpectedType, p.Source, "tasks.FinishedTask")
	}

	return ts.ClientApparentAddr, nil

}
func FinishedTask__MinerLatencyMS__resolve(p graphql.ResolveParams) (interface{}, error) {
	ts, ok := p.Source.(*tasks.FinishedTask)
	if !ok {
		return nil, fmt.Errorf(errUnexpectedType, p.Source, "tasks.FinishedTask")
	}

	f := ts.MinerLatencyMS
	if f != nil {

		return *f, nil

	} else {
		return nil, nil
	}

}
func FinishedTask__TimeToFirstByteMS__resolve(p graphql.ResolveParams) (interface{}, error) {
	ts, ok := p.Source.(*tasks.FinishedTask)
	if !ok {
		return nil, fmt.Errorf(errUnexpectedType, p.Source, "tasks.FinishedTask")
	}

	f := ts.TimeToFirstByteMS
	if f != nil {

		return *f, nil

	} else {
		return nil, nil
	}

}
func FinishedTask__TimeToLastByteMS__resolve(p graphql.ResolveParams) (interface{}, error) {
	ts, ok := p.Source.(*tasks.FinishedTask)
	if !ok {
		return nil, fmt.Errorf(errUnexpectedType, p.Source, "tasks.FinishedTask")
	}

	f := ts.TimeToLastByteMS
	if f != nil {

		return *f, nil

	} else {
		return nil, nil
	}

}
func FinishedTask__Events__resolve(p graphql.ResolveParams) (interface{}, error) {
	ts, ok := p.Source.(*tasks.FinishedTask)
	if !ok {
		return nil, fmt.Errorf(errUnexpectedType, p.Source, "tasks.FinishedTask")
	}

	targetCid := ts.Events

	var node ipld.Node

	if cl, ok := targetCid.(cidlink.Link); ok {
		v := p.Context.Value(nodeLoaderCtxKey)
		if v == nil {
			return cl.Cid, nil
		}
		loader, ok := v.(func(context.Context, cidlink.Link, ipld.NodeBuilder) (ipld.Node, error))
		if !ok {
			return nil, errInvalidLoader
		}

		builder := tasks.StageDetailsListPrototype.NewBuilder()
		n, err := loader(p.Context, cl, builder)
		if err != nil {
			return nil, err
		}
		node = n
	} else {
		return nil, errInvalidLink
	}

	return node, nil

}
func FinishedTask__MinerVersion__resolve(p graphql.ResolveParams) (interface{}, error) {
	ts, ok := p.Source.(*tasks.FinishedTask)
	if !ok {
		return nil, fmt.Errorf(errUnexpectedType, p.Source, "tasks.FinishedTask")
	}

	f := ts.MinerVersion
	if f != nil {

		return *f, nil

	} else {
		return nil, nil
	}

}
func FinishedTask__ClientVersion__resolve(p graphql.ResolveParams) (interface{}, error) {
	ts, ok := p.Source.(*tasks.FinishedTask)
	if !ok {
		return nil, fmt.Errorf(errUnexpectedType, p.Source, "tasks.FinishedTask")
	}

	f := ts.ClientVersion
	if f != nil {

		return *f, nil

	} else {
		return nil, nil
	}

}
func FinishedTask__Size__resolve(p graphql.ResolveParams) (interface{}, error) {
	ts, ok := p.Source.(*tasks.FinishedTask)
	if !ok {
		return nil, fmt.Errorf(errUnexpectedType, p.Source, "tasks.FinishedTask")
	}

	f := ts.Size
	if f != nil {

		return *f, nil

	} else {
		return nil, nil
	}

}
func FinishedTask__PayloadCID__resolve(p graphql.ResolveParams) (interface{}, error) {
	ts, ok := p.Source.(*tasks.FinishedTask)
	if !ok {
		return nil, fmt.Errorf(errUnexpectedType, p.Source, "tasks.FinishedTask")
	}

	f := ts.PayloadCID
	if f != nil {

		return *f, nil

	} else {
		return nil, nil
	}

}
func FinishedTask__ProposalCID__resolve(p graphql.ResolveParams) (interface{}, error) {
	ts, ok := p.Source.(*tasks.FinishedTask)
	if !ok {
		return nil, fmt.Errorf(errUnexpectedType, p.Source, "tasks.FinishedTask")
	}

	f := ts.ProposalCID
	if f != nil {

		return *f, nil

	} else {
		return nil, nil
	}

}
func FinishedTask__DealIDString__resolve(p graphql.ResolveParams) (interface{}, error) {
	ts, ok := p.Source.(*tasks.FinishedTask)
	if !ok {
		return nil, fmt.Errorf(errUnexpectedType, p.Source, "tasks.FinishedTask")
	}

	f := ts.DealIDString
	if f != nil {

		return *f, nil

	} else {
		return nil, nil
	}

}
func FinishedTask__MinerPeerID__resolve(p graphql.ResolveParams) (interface{}, error) {
	ts, ok := p.Source.(*tasks.FinishedTask)
	if !ok {
		return nil, fmt.Errorf(errUnexpectedType, p.Source, "tasks.FinishedTask")
	}

	f := ts.MinerPeerID
	if f != nil {

		return *f, nil

	} else {
		return nil, nil
	}

}

var FinishedTask__type = graphql.NewObject(graphql.ObjectConfig{
	Name: "FinishedTask",
	Fields: graphql.Fields{
		"Status": &graphql.Field{

			Type: graphql.NewNonNull(graphql.Int),

			Resolve: FinishedTask__Status__resolve,
		},
		"StartedAt": &graphql.Field{

			Type: graphql.NewNonNull(graphql.Int),

			Resolve: FinishedTask__StartedAt__resolve,
		},
		"ErrorMessage": &graphql.Field{

			Type: graphql.String,

			Resolve: FinishedTask__ErrorMessage__resolve,
		},
		"RetrievalTask": &graphql.Field{

			Type: RetrievalTask__type,

			Resolve: FinishedTask__RetrievalTask__resolve,
		},
		"StorageTask": &graphql.Field{

			Type: StorageTask__type,

			Resolve: FinishedTask__StorageTask__resolve,
		},
		"DealID": &graphql.Field{

			Type: graphql.NewNonNull(graphql.Int),

			Resolve: FinishedTask__DealID__resolve,
		},
		"MinerMultiAddr": &graphql.Field{

			Type: graphql.NewNonNull(graphql.String),

			Resolve: FinishedTask__MinerMultiAddr__resolve,
		},
		"ClientApparentAddr": &graphql.Field{

			Type: graphql.NewNonNull(graphql.String),

			Resolve: FinishedTask__ClientApparentAddr__resolve,
		},
		"MinerLatencyMS": &graphql.Field{

			Type: graphql.Int,

			Resolve: FinishedTask__MinerLatencyMS__resolve,
		},
		"TimeToFirstByteMS": &graphql.Field{

			Type: graphql.Int,

			Resolve: FinishedTask__TimeToFirstByteMS__resolve,
		},
		"TimeToLastByteMS": &graphql.Field{

			Type: graphql.Int,

			Resolve: FinishedTask__TimeToLastByteMS__resolve,
		},
		"Events": &graphql.Field{

			Type: graphql.NewNonNull(List_StageDetails__type),

			Resolve: FinishedTask__Events__resolve,
		},
		"MinerVersion": &graphql.Field{

			Type: graphql.String,

			Resolve: FinishedTask__MinerVersion__resolve,
		},
		"ClientVersion": &graphql.Field{

			Type: graphql.String,

			Resolve: FinishedTask__ClientVersion__resolve,
		},
		"Size": &graphql.Field{

			Type: graphql.Int,

			Resolve: FinishedTask__Size__resolve,
		},
		"PayloadCID": &graphql.Field{

			Type: graphql.String,

			Resolve: FinishedTask__PayloadCID__resolve,
		},
		"ProposalCID": &graphql.Field{

			Type: graphql.String,

			Resolve: FinishedTask__ProposalCID__resolve,
		},
		"DealIDString": &graphql.Field{

			Type: graphql.String,

			Resolve: FinishedTask__DealIDString__resolve,
		},
		"MinerPeerID": &graphql.Field{

			Type: graphql.String,

			Resolve: FinishedTask__MinerPeerID__resolve,
		},
	},
})
var FinishedTasks__type = graphql.NewObject(graphql.ObjectConfig{
	Name: "FinishedTasks",
	Fields: graphql.Fields{
		"At": &graphql.Field{
			Type: FinishedTask__type,
			Args: graphql.FieldConfigArgument{
				"key": &graphql.ArgumentConfig{
					Type: graphql.NewNonNull(graphql.Int),
				},
			},
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				ts, ok := p.Source.(*tasks.FinishedTasks)
				if !ok {
					return nil, errWrongKindRes
				}

				arg := p.Args["key"]
				var out *tasks.FinishedTask
				var err error
				switch ta := arg.(type) {
				//case ipld.Node:
				//    out, err = ts.LookupByNode(ta)
				case int64:
					l := int64(len(*ts))
					if ta >= l {
						err = fmt.Errorf("out of range")
					}
					out = &((*ts)[ta])
				default:
					return nil, fmt.Errorf("unknown key type: %T", arg)
				}

				return out, err

			},
		},
		"All": &graphql.Field{
			Type: graphql.NewList(FinishedTask__type),
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				ts, ok := p.Source.(*tasks.FinishedTasks)
				if !ok {
					return nil, errWrongKindRes
				}
				//it := ts.ListIterator()
				children := make([]*tasks.FinishedTask, 0)
				for _, fts := range *ts {
					children = append(children, &fts)
				}
				return children, nil
			},
		},
		//"Range": &graphql.Field{
		//    Type: graphql.NewList(FinishedTask__type),
		//    Args: graphql.FieldConfigArgument{
		//        "skip": &graphql.ArgumentConfig{
		//            Type: graphql.NewNonNull(graphql.Int),
		//        },
		//        "take": &graphql.ArgumentConfig{
		//            Type: graphql.NewNonNull(graphql.Int),
		//        },
		//    },
		//    Resolve: func(p graphql.ResolveParams) (interface{}, error) {
		//        ts, ok := p.Source.(tasks.FinishedTasks)
		//        if !ok {
		//            return nil, errNotNode
		//        }
		//        it := ts.ListIterator()
		//        children := make([]ipld.Node, 0)
		//
		//        for !it.Done() {
		//            _, node, err := it.Next()
		//            if err != nil {
		//                return nil, err
		//            }
		//
		//            children = append(children, node)
		//        }
		//        return children, nil
		//    },
		//},
		"Count": &graphql.Field{
			Type: graphql.NewNonNull(graphql.Int),
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				ts, ok := p.Source.(*tasks.FinishedTasks)
				if !ok {
					return nil, errWrongKindRes
				}
				return int64(len(*ts)), nil
			},
		},
	},
})
var List_AuthenticatedRecord__type = graphql.NewObject(graphql.ObjectConfig{
	Name: "List_AuthenticatedRecord",
	Fields: graphql.Fields{
		"At": &graphql.Field{
			Type: AuthenticatedRecord__type,
			Args: graphql.FieldConfigArgument{
				"key": &graphql.ArgumentConfig{
					Type: graphql.NewNonNull(graphql.Int),
				},
			},
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				ts, ok := p.Source.(*tasks.AuthenticatedRecordList)
				if !ok {
					return nil, errWrongKindRes
				}

				arg := p.Args["key"]
				var out *tasks.AuthenticatedRecord
				var err error
				switch ta := arg.(type) {
				case int64:
					l := int64(len(*ts))
					if ta >= l {
						err = fmt.Errorf("out of range")
					}
					out = &((*ts)[ta])
				default:
					return nil, fmt.Errorf("unknown key type: %T", arg)
				}

				return out, err

			},
		},
		"All": &graphql.Field{
			Type: graphql.NewList(AuthenticatedRecord__type),
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				ts, ok := p.Source.(*tasks.AuthenticatedRecordList)
				if !ok {
					return nil, errWrongKindRes
				}
				children := make([]*tasks.AuthenticatedRecord, 0)
				for _, ar := range *ts {
					children = append(children, &ar)
				}
				return children, nil
			},
		},
		//"Range": &graphql.Field{
		//    Type: graphql.NewList(AuthenticatedRecord__type),
		//    Args: graphql.FieldConfigArgument{
		//        "skip": &graphql.ArgumentConfig{
		//            Type: graphql.NewNonNull(graphql.Int),
		//        },
		//        "take": &graphql.ArgumentConfig{
		//            Type: graphql.NewNonNull(graphql.Int),
		//        },
		//    },
		//    Resolve: func(p graphql.ResolveParams) (interface{}, error) {
		//        ts, ok := p.Source.(tasks.List_AuthenticatedRecord)
		//        if !ok {
		//            return nil, errNotNode
		//        }
		//        it := ts.ListIterator()
		//        children := make([]ipld.Node, 0)
		//
		//        for !it.Done() {
		//            _, node, err := it.Next()
		//            if err != nil {
		//                return nil, err
		//            }
		//
		//            children = append(children, node)
		//        }
		//        return children, nil
		//    },
		//},
		"Count": &graphql.Field{
			Type: graphql.NewNonNull(graphql.Int),
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				ts, ok := p.Source.(*tasks.AuthenticatedRecordList)
				if !ok {
					return nil, errWrongKindRes
				}
				return int64(len(*ts)), nil
			},
		},
	},
})
var List_Logs__type = graphql.NewObject(graphql.ObjectConfig{
	Name: "List_Logs",
	Fields: graphql.Fields{
		"At": &graphql.Field{
			Type: Logs__type,
			Args: graphql.FieldConfigArgument{
				"key": &graphql.ArgumentConfig{
					Type: graphql.NewNonNull(graphql.Int),
				},
			},
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				ts, ok := p.Source.(*[]tasks.Logs)
				if !ok {
					return nil, errWrongKindRes
				}

				arg := p.Args["key"]
				var out *tasks.Logs
				var err error
				switch ta := arg.(type) {
				case int64:
					l := int64(len(*ts))
					if ta >= l {
						err = fmt.Errorf("out of range")
					}
					out = &(*ts)[ta]
				default:
					return nil, fmt.Errorf("unknown key type: %T", arg)
				}

				return out, err

			},
		},
		"All": &graphql.Field{
			Type: graphql.NewList(Logs__type),
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				ts, ok := p.Source.(*[]tasks.Logs)
				if !ok {
					return nil, errWrongKindRes
				}
				children := make([]*tasks.Logs, 0)
				for _, l := range *ts {
					children = append(children, &l)
				}
				return children, nil
			},
		},
		//"Range": &graphql.Field{
		//    Type: graphql.NewList(Logs__type),
		//    Args: graphql.FieldConfigArgument{
		//        "skip": &graphql.ArgumentConfig{
		//            Type: graphql.NewNonNull(graphql.Int),
		//        },
		//        "take": &graphql.ArgumentConfig{
		//            Type: graphql.NewNonNull(graphql.Int),
		//        },
		//    },
		//    Resolve: func(p graphql.ResolveParams) (interface{}, error) {
		//        ts, ok := p.Source.(tasks.List_Logs)
		//        if !ok {
		//            return nil, errNotNode
		//        }
		//        it := ts.ListIterator()
		//        children := make([]ipld.Node, 0)
		//
		//        for !it.Done() {
		//            _, node, err := it.Next()
		//            if err != nil {
		//                return nil, err
		//            }
		//
		//            children = append(children, node)
		//        }
		//        return children, nil
		//    },
		//},
		"Count": &graphql.Field{
			Type: graphql.NewNonNull(graphql.Int),
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				ts, ok := p.Source.(*[]tasks.Logs)
				if !ok {
					return nil, errWrongKindRes
				}
				return int64(len(*ts)), nil
			},
		},
	},
})
var List_StageDetails__type = graphql.NewObject(graphql.ObjectConfig{
	Name: "List_StageDetails",
	Fields: graphql.Fields{
		"At": &graphql.Field{
			Type: StageDetails__type,
			Args: graphql.FieldConfigArgument{
				"key": &graphql.ArgumentConfig{
					Type: graphql.NewNonNull(graphql.Int),
				},
			},
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				ts, ok := p.Source.(*tasks.StageDetailsList)
				if !ok {
					return nil, errWrongKindRes
				}

				arg := p.Args["key"]
				var out *tasks.StageDetails
				var err error
				switch ta := arg.(type) {
				case int64:
					l := int64(len(*ts))
					if ta >= l {
						err = fmt.Errorf("out of range")
					}
					out = &((*ts)[ta])
				default:
					return nil, fmt.Errorf("unknown key type: %T", arg)
				}

				return out, err

			},
		},
		"All": &graphql.Field{
			Type: graphql.NewList(StageDetails__type),
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				ts, ok := p.Source.(*tasks.StageDetailsList)
				if !ok {
					return nil, errWrongKindRes
				}
				children := make([]*tasks.StageDetails, 0)
				for _, sd := range *ts {
					children = append(children, &sd)
				}
				return children, nil
			},
		},
		//"Range": &graphql.Field{
		//    Type: graphql.NewList(StageDetails__type),
		//    Args: graphql.FieldConfigArgument{
		//        "skip": &graphql.ArgumentConfig{
		//            Type: graphql.NewNonNull(graphql.Int),
		//        },
		//        "take": &graphql.ArgumentConfig{
		//            Type: graphql.NewNonNull(graphql.Int),
		//        },
		//    },
		//    Resolve: func(p graphql.ResolveParams) (interface{}, error) {
		//        ts, ok := p.Source.(tasks.List_StageDetails)
		//        if !ok {
		//            return nil, errNotNode
		//        }
		//        it := ts.ListIterator()
		//        children := make([]ipld.Node, 0)
		//
		//        for !it.Done() {
		//            _, node, err := it.Next()
		//            if err != nil {
		//                return nil, err
		//            }
		//
		//            children = append(children, node)
		//        }
		//        return children, nil
		//    },
		//},
		"Count": &graphql.Field{
			Type: graphql.NewNonNull(graphql.Int),
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				ts, ok := p.Source.(*tasks.StageDetailsList)
				if !ok {
					return nil, errWrongKindRes
				}
				return int64(len(*ts)), nil
			},
		},
	},
})

var List_String__type = graphql.NewObject(graphql.ObjectConfig{
	Name: "List_String",
	Fields: graphql.Fields{
		"At": &graphql.Field{
			Type: graphql.String,
			Args: graphql.FieldConfigArgument{
				"key": &graphql.ArgumentConfig{
					Type: graphql.NewNonNull(graphql.Int),
				},
			},
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				ts, ok := p.Source.(*[]string)
				if !ok {
					return nil, errWrongKindRes
				}

				arg := p.Args["key"]
				var out *string
				var err error
				switch ta := arg.(type) {
				case int64:
					l := int64(len(*ts))
					if ta >= l {
						err = fmt.Errorf("out of range")
					}
					out = &((*ts)[ta])
				default:
					return nil, fmt.Errorf("unknown key type: %T", arg)
				}

				return out, err

			},
		},
		"All": &graphql.Field{
			Type: graphql.NewList(graphql.String),
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				ts, ok := p.Source.(*[]string)
				if !ok {
					return nil, errWrongKindRes
				}
				children := make([]*string, 0)
				for _, str := range *ts {
					children = append(children, &str)
				}
				return children, nil
			},
		},
		//"Range": &graphql.Field{
		//    Type: graphql.NewList(graphql.String),
		//    Args: graphql.FieldConfigArgument{
		//        "skip": &graphql.ArgumentConfig{
		//            Type: graphql.NewNonNull(graphql.Int),
		//        },
		//        "take": &graphql.ArgumentConfig{
		//            Type: graphql.NewNonNull(graphql.Int),
		//        },
		//    },
		//    Resolve: func(p graphql.ResolveParams) (interface{}, error) {
		//        ts, ok := p.Source.(tasks.List_String)
		//        if !ok {
		//            return nil, errNotNode
		//        }
		//        it := ts.ListIterator()
		//        children := make([]ipld.Node, 0)
		//
		//        for !it.Done() {
		//            _, node, err := it.Next()
		//            if err != nil {
		//                return nil, err
		//            }
		//
		//            children = append(children, node)
		//        }
		//        return children, nil
		//    },
		//},
		"Count": &graphql.Field{
			Type: graphql.NewNonNull(graphql.Int),
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				ts, ok := p.Source.(*[]string)
				if !ok {
					return nil, errWrongKindRes
				}
				return int64(len(*ts)), nil
			},
		},
	},
})

func Logs__Log__resolve(p graphql.ResolveParams) (interface{}, error) {
	ts, ok := p.Source.(*tasks.Logs)
	if !ok {
		return nil, fmt.Errorf(errUnexpectedType, p.Source, "tasks.Logs")
	}

	return ts.Log, nil

}
func Logs__UpdatedAt__resolve(p graphql.ResolveParams) (interface{}, error) {
	ts, ok := p.Source.(*tasks.Logs)
	if !ok {
		return nil, fmt.Errorf(errUnexpectedType, p.Source, "tasks.Logs")
	}

	return int64(ts.UpdatedAt), nil

}

var Logs__type = graphql.NewObject(graphql.ObjectConfig{
	Name: "Logs",
	Fields: graphql.Fields{
		"Log": &graphql.Field{

			Type: graphql.NewNonNull(graphql.String),

			Resolve: Logs__Log__resolve,
		},
		"UpdatedAt": &graphql.Field{

			Type: graphql.NewNonNull(graphql.Int),

			Resolve: Logs__UpdatedAt__resolve,
		},
	},
})

func PopTask__Status__resolve(p graphql.ResolveParams) (interface{}, error) {
	ts, ok := p.Source.(*tasks.PopTask)
	if !ok {
		return nil, fmt.Errorf(errUnexpectedType, p.Source, "tasks.PopTask")
	}

	return int64(ts.Status), nil

}
func PopTask__WorkedBy__resolve(p graphql.ResolveParams) (interface{}, error) {
	ts, ok := p.Source.(*tasks.PopTask)
	if !ok {
		return nil, fmt.Errorf(errUnexpectedType, p.Source, "tasks.PopTask")
	}

	return ts.WorkedBy, nil

}
func PopTask__Tags__resolve(p graphql.ResolveParams) (interface{}, error) {
	ts, ok := p.Source.(*tasks.PopTask)
	if !ok {
		return nil, fmt.Errorf(errUnexpectedType, p.Source, "tasks.PopTask")
	}

	f := ts.Tags
	if f != nil {

		return &f, nil

	} else {
		return nil, nil
	}

}

var PopTask__type = graphql.NewObject(graphql.ObjectConfig{
	Name: "PopTask",
	Fields: graphql.Fields{
		"Status": &graphql.Field{

			Type: graphql.NewNonNull(graphql.Int),

			Resolve: PopTask__Status__resolve,
		},
		"WorkedBy": &graphql.Field{

			Type: graphql.NewNonNull(graphql.String),

			Resolve: PopTask__WorkedBy__resolve,
		},
		"Tags": &graphql.Field{

			Type: List_String__type,

			Resolve: PopTask__Tags__resolve,
		},
	},
})

func RecordUpdate__Records__resolve(p graphql.ResolveParams) (interface{}, error) {
	ts, ok := p.Source.(*tasks.RecordUpdate)
	if !ok {
		return nil, fmt.Errorf(errUnexpectedType, p.Source, "tasks.RecordUpdate")
	}

	return ts.Records, nil

}
func RecordUpdate__SigPrev__resolve(p graphql.ResolveParams) (interface{}, error) {
	ts, ok := p.Source.(*tasks.RecordUpdate)
	if !ok {
		return nil, fmt.Errorf(errUnexpectedType, p.Source, "tasks.RecordUpdate")
	}

	return string(ts.SigPrev), nil

}
func RecordUpdate__Previous__resolve(p graphql.ResolveParams) (interface{}, error) {
	ts, ok := p.Source.(*tasks.RecordUpdate)
	if !ok {
		return nil, fmt.Errorf(errUnexpectedType, p.Source, "tasks.RecordUpdate")
	}

	f := ts.Previous
	if f != nil {

		return f, nil

	} else {
		return nil, nil
	}

}

var RecordUpdate__type = graphql.NewObject(graphql.ObjectConfig{
	Name: "RecordUpdate",
	Fields: graphql.Fields{
		"Records": &graphql.Field{

			Type: graphql.NewNonNull(List_AuthenticatedRecord__type),

			Resolve: RecordUpdate__Records__resolve,
		},
		"SigPrev": &graphql.Field{

			Type: graphql.NewNonNull(graphql.String),

			Resolve: RecordUpdate__SigPrev__resolve,
		},
		"Previous": &graphql.Field{

			Type: graphql.ID,

			Resolve: RecordUpdate__Previous__resolve,
		},
	},
})

func RetrievalTask__Miner__resolve(p graphql.ResolveParams) (interface{}, error) {
	ts, ok := p.Source.(*tasks.RetrievalTask)
	if !ok {
		return nil, fmt.Errorf(errUnexpectedType, p.Source, "tasks.RetrievalTask")
	}

	return ts.Miner, nil

}
func RetrievalTask__PayloadCID__resolve(p graphql.ResolveParams) (interface{}, error) {
	ts, ok := p.Source.(*tasks.RetrievalTask)
	if !ok {
		return nil, fmt.Errorf(errUnexpectedType, p.Source, "tasks.RetrievalTask")
	}

	return ts.PayloadCID, nil

}
func RetrievalTask__CARExport__resolve(p graphql.ResolveParams) (interface{}, error) {
	ts, ok := p.Source.(*tasks.RetrievalTask)
	if !ok {
		return nil, fmt.Errorf(errUnexpectedType, p.Source, "tasks.RetrievalTask")
	}

	return ts.CARExport, nil

}
func RetrievalTask__Schedule__resolve(p graphql.ResolveParams) (interface{}, error) {
	ts, ok := p.Source.(*tasks.RetrievalTask)
	if !ok {
		return nil, fmt.Errorf(errUnexpectedType, p.Source, "tasks.RetrievalTask")
	}

	f := ts.Schedule
	if f != nil {

		return *f, nil

	} else {
		return nil, nil
	}

}
func RetrievalTask__ScheduleLimit__resolve(p graphql.ResolveParams) (interface{}, error) {
	ts, ok := p.Source.(*tasks.RetrievalTask)
	if !ok {
		return nil, fmt.Errorf(errUnexpectedType, p.Source, "tasks.RetrievalTask")
	}

	f := ts.ScheduleLimit
	if f != nil {

		return *f, nil

	} else {
		return nil, nil
	}

}
func RetrievalTask__Tag__resolve(p graphql.ResolveParams) (interface{}, error) {
	ts, ok := p.Source.(*tasks.RetrievalTask)
	if !ok {
		return nil, fmt.Errorf(errUnexpectedType, p.Source, "tasks.RetrievalTask")
	}

	f := ts.Tag
	if f != nil {

		return *f, nil

	} else {
		return nil, nil
	}

}
func RetrievalTask__MaxPriceAttoFIL__resolve(p graphql.ResolveParams) (interface{}, error) {
	ts, ok := p.Source.(*tasks.RetrievalTask)
	if !ok {
		return nil, fmt.Errorf(errUnexpectedType, p.Source, "tasks.RetrievalTask")
	}

	f := ts.MaxPriceAttoFIL
	if f != nil {

		return *f, nil

	} else {
		return nil, nil
	}

}

var RetrievalTask__type = graphql.NewObject(graphql.ObjectConfig{
	Name: "RetrievalTask",
	Fields: graphql.Fields{
		"Miner": &graphql.Field{

			Type: graphql.NewNonNull(graphql.String),

			Resolve: RetrievalTask__Miner__resolve,
		},
		"PayloadCID": &graphql.Field{

			Type: graphql.NewNonNull(graphql.String),

			Resolve: RetrievalTask__PayloadCID__resolve,
		},
		"CARExport": &graphql.Field{

			Type: graphql.NewNonNull(graphql.Boolean),

			Resolve: RetrievalTask__CARExport__resolve,
		},
		"Schedule": &graphql.Field{

			Type: graphql.String,

			Resolve: RetrievalTask__Schedule__resolve,
		},
		"ScheduleLimit": &graphql.Field{

			Type: graphql.String,

			Resolve: RetrievalTask__ScheduleLimit__resolve,
		},
		"Tag": &graphql.Field{

			Type: graphql.String,

			Resolve: RetrievalTask__Tag__resolve,
		},
		"MaxPriceAttoFIL": &graphql.Field{

			Type: graphql.Int,

			Resolve: RetrievalTask__MaxPriceAttoFIL__resolve,
		},
	},
})

func StageDetails__Description__resolve(p graphql.ResolveParams) (interface{}, error) {
	ts, ok := p.Source.(*tasks.StageDetails)
	if !ok {
		return nil, fmt.Errorf(errUnexpectedType, p.Source, "tasks.StageDetails")
	}

	f := ts.Description
	if f != nil {

		return f, nil

	} else {
		return nil, nil
	}

}
func StageDetails__ExpectedDuration__resolve(p graphql.ResolveParams) (interface{}, error) {
	ts, ok := p.Source.(*tasks.StageDetails)
	if !ok {
		return nil, fmt.Errorf(errUnexpectedType, p.Source, "tasks.StageDetails")
	}

	f := ts.ExpectedDuration
	if f != nil {

		return f, nil

	} else {
		return nil, nil
	}

}
func StageDetails__Logs__resolve(p graphql.ResolveParams) (interface{}, error) {
	ts, ok := p.Source.(*tasks.StageDetails)
	if !ok {
		return nil, fmt.Errorf(errUnexpectedType, p.Source, "tasks.StageDetails")
	}

	return ts.Logs, nil

}
func StageDetails__UpdatedAt__resolve(p graphql.ResolveParams) (interface{}, error) {
	ts, ok := p.Source.(*tasks.StageDetails)
	if !ok {
		return nil, fmt.Errorf(errUnexpectedType, p.Source, "tasks.StageDetails")
	}

	f := ts.UpdatedAt
	if f != nil {

		return f, nil

	} else {
		return nil, nil
	}

}

var StageDetails__type = graphql.NewObject(graphql.ObjectConfig{
	Name: "StageDetails",
	Fields: graphql.Fields{
		"Description": &graphql.Field{

			Type: graphql.String,

			Resolve: StageDetails__Description__resolve,
		},
		"ExpectedDuration": &graphql.Field{

			Type: graphql.String,

			Resolve: StageDetails__ExpectedDuration__resolve,
		},
		"Logs": &graphql.Field{

			Type: graphql.NewNonNull(List_Logs__type),

			Resolve: StageDetails__Logs__resolve,
		},
		"UpdatedAt": &graphql.Field{

			Type: graphql.Int,

			Resolve: StageDetails__UpdatedAt__resolve,
		},
	},
})

func StorageTask__Miner__resolve(p graphql.ResolveParams) (interface{}, error) {
	ts, ok := p.Source.(*tasks.StorageTask)
	if !ok {
		return nil, fmt.Errorf(errUnexpectedType, p.Source, "tasks.StorageTask")
	}

	return ts.Miner, nil

}
func StorageTask__MaxPriceAttoFIL__resolve(p graphql.ResolveParams) (interface{}, error) {
	ts, ok := p.Source.(*tasks.StorageTask)
	if !ok {
		return nil, fmt.Errorf(errUnexpectedType, p.Source, "tasks.StorageTask")
	}

	return ts.MaxPriceAttoFIL, nil

}
func StorageTask__Size__resolve(p graphql.ResolveParams) (interface{}, error) {
	ts, ok := p.Source.(*tasks.StorageTask)
	if !ok {
		return nil, fmt.Errorf(errUnexpectedType, p.Source, "tasks.StorageTask")
	}

	return ts.Size, nil

}
func StorageTask__StartOffset__resolve(p graphql.ResolveParams) (interface{}, error) {
	ts, ok := p.Source.(*tasks.StorageTask)
	if !ok {
		return nil, fmt.Errorf(errUnexpectedType, p.Source, "tasks.StorageTask")
	}

	return ts.StartOffset, nil

}
func StorageTask__FastRetrieval__resolve(p graphql.ResolveParams) (interface{}, error) {
	ts, ok := p.Source.(*tasks.StorageTask)
	if !ok {
		return nil, fmt.Errorf(errUnexpectedType, p.Source, "tasks.StorageTask")
	}

	return ts.FastRetrieval, nil

}
func StorageTask__Verified__resolve(p graphql.ResolveParams) (interface{}, error) {
	ts, ok := p.Source.(*tasks.StorageTask)
	if !ok {
		return nil, fmt.Errorf(errUnexpectedType, p.Source, "tasks.StorageTask")
	}

	return ts.Verified, nil

}
func StorageTask__Schedule__resolve(p graphql.ResolveParams) (interface{}, error) {
	ts, ok := p.Source.(*tasks.StorageTask)
	if !ok {
		return nil, fmt.Errorf(errUnexpectedType, p.Source, "tasks.StorageTask")
	}

	f := ts.Schedule
	if f != nil {

		return *f, nil

	} else {
		return nil, nil
	}

}
func StorageTask__ScheduleLimit__resolve(p graphql.ResolveParams) (interface{}, error) {
	ts, ok := p.Source.(*tasks.StorageTask)
	if !ok {
		return nil, fmt.Errorf(errUnexpectedType, p.Source, "tasks.StorageTask")
	}

	f := ts.ScheduleLimit
	if f != nil {

		return *f, nil

	} else {
		return nil, nil
	}

}
func StorageTask__Tag__resolve(p graphql.ResolveParams) (interface{}, error) {
	ts, ok := p.Source.(*tasks.StorageTask)
	if !ok {
		return nil, fmt.Errorf(errUnexpectedType, p.Source, "tasks.StorageTask")
	}

	f := ts.Tag
	if f != nil {

		return *f, nil

	} else {
		return nil, nil
	}

}
func StorageTask__RetrievalSchedule__resolve(p graphql.ResolveParams) (interface{}, error) {
	ts, ok := p.Source.(*tasks.StorageTask)
	if !ok {
		return nil, fmt.Errorf(errUnexpectedType, p.Source, "tasks.StorageTask")
	}

	f := ts.RetrievalSchedule
	if f != nil {

		return *f, nil

	} else {
		return nil, nil
	}

}
func StorageTask__RetrievalScheduleLimit__resolve(p graphql.ResolveParams) (interface{}, error) {
	ts, ok := p.Source.(*tasks.StorageTask)
	if !ok {
		return nil, fmt.Errorf(errUnexpectedType, p.Source, "tasks.StorageTask")
	}

	f := ts.RetrievalScheduleLimit
	if f != nil {

		return *f, nil

	} else {
		return nil, nil
	}

}
func StorageTask__RetrievalMaxPriceAttoFIL__resolve(p graphql.ResolveParams) (interface{}, error) {
	ts, ok := p.Source.(*tasks.StorageTask)
	if !ok {
		return nil, fmt.Errorf(errUnexpectedType, p.Source, "tasks.StorageTask")
	}

	f := ts.RetrievalMaxPriceAttoFIL
	if f != nil {

		return *f, nil

	} else {
		return nil, nil
	}

}

var StorageTask__type = graphql.NewObject(graphql.ObjectConfig{
	Name: "StorageTask",
	Fields: graphql.Fields{
		"Miner": &graphql.Field{

			Type: graphql.NewNonNull(graphql.String),

			Resolve: StorageTask__Miner__resolve,
		},
		"MaxPriceAttoFIL": &graphql.Field{

			Type: graphql.NewNonNull(graphql.Int),

			Resolve: StorageTask__MaxPriceAttoFIL__resolve,
		},
		"Size": &graphql.Field{

			Type: graphql.NewNonNull(graphql.Int),

			Resolve: StorageTask__Size__resolve,
		},
		"StartOffset": &graphql.Field{

			Type: graphql.NewNonNull(graphql.Int),

			Resolve: StorageTask__StartOffset__resolve,
		},
		"FastRetrieval": &graphql.Field{

			Type: graphql.NewNonNull(graphql.Boolean),

			Resolve: StorageTask__FastRetrieval__resolve,
		},
		"Verified": &graphql.Field{

			Type: graphql.NewNonNull(graphql.Boolean),

			Resolve: StorageTask__Verified__resolve,
		},
		"Schedule": &graphql.Field{

			Type: graphql.String,

			Resolve: StorageTask__Schedule__resolve,
		},
		"ScheduleLimit": &graphql.Field{

			Type: graphql.String,

			Resolve: StorageTask__ScheduleLimit__resolve,
		},
		"Tag": &graphql.Field{

			Type: graphql.String,

			Resolve: StorageTask__Tag__resolve,
		},
		"RetrievalSchedule": &graphql.Field{

			Type: graphql.String,

			Resolve: StorageTask__RetrievalSchedule__resolve,
		},
		"RetrievalScheduleLimit": &graphql.Field{

			Type: graphql.String,

			Resolve: StorageTask__RetrievalScheduleLimit__resolve,
		},
		"RetrievalMaxPriceAttoFIL": &graphql.Field{

			Type: graphql.Int,

			Resolve: StorageTask__RetrievalMaxPriceAttoFIL__resolve,
		},
	},
})

func Task__UUID__resolve(p graphql.ResolveParams) (interface{}, error) {
	ts, ok := p.Source.(*tasks.Task)
	if !ok {
		return nil, fmt.Errorf(errUnexpectedType, p.Source, "tasks.Task")
	}

	return ts.UUID, nil

}
func Task__Status__resolve(p graphql.ResolveParams) (interface{}, error) {
	ts, ok := p.Source.(*tasks.Task)
	if !ok {
		return nil, fmt.Errorf(errUnexpectedType, p.Source, "tasks.Task")
	}

	return int64(ts.Status), nil

}
func Task__WorkedBy__resolve(p graphql.ResolveParams) (interface{}, error) {
	ts, ok := p.Source.(*tasks.Task)
	if !ok {
		return nil, fmt.Errorf(errUnexpectedType, p.Source, "tasks.Task")
	}

	f := ts.WorkedBy
	if f != nil {

		return f, nil

	} else {
		return nil, nil
	}

}
func Task__Stage__resolve(p graphql.ResolveParams) (interface{}, error) {
	ts, ok := p.Source.(*tasks.Task)
	if !ok {
		return nil, fmt.Errorf(errUnexpectedType, p.Source, "tasks.Task")
	}

	return ts.Stage, nil

}
func Task__CurrentStageDetails__resolve(p graphql.ResolveParams) (interface{}, error) {
	ts, ok := p.Source.(*tasks.Task)
	if !ok {
		return nil, fmt.Errorf(errUnexpectedType, p.Source, "tasks.Task")
	}

	f := ts.CurrentStageDetails
	if f != nil {

		return f, nil

	} else {
		return nil, nil
	}

}
func Task__PastStageDetails__resolve(p graphql.ResolveParams) (interface{}, error) {
	ts, ok := p.Source.(*tasks.Task)
	if !ok {
		return nil, fmt.Errorf(errUnexpectedType, p.Source, "tasks.Task")
	}

	f := ts.PastStageDetails
	if f != nil {

		return f, nil

	} else {
		return nil, nil
	}

}
func Task__StartedAt__resolve(p graphql.ResolveParams) (interface{}, error) {
	ts, ok := p.Source.(*tasks.Task)
	if !ok {
		return nil, fmt.Errorf(errUnexpectedType, p.Source, "tasks.Task")
	}

	f := ts.StartedAt
	if f != nil {

		return *f, nil

	} else {
		return nil, nil
	}

}
func Task__RunCount__resolve(p graphql.ResolveParams) (interface{}, error) {
	ts, ok := p.Source.(*tasks.Task)
	if !ok {
		return nil, fmt.Errorf(errUnexpectedType, p.Source, "tasks.Task")
	}

	return ts.RunCount, nil

}
func Task__ErrorMessage__resolve(p graphql.ResolveParams) (interface{}, error) {
	ts, ok := p.Source.(*tasks.Task)
	if !ok {
		return nil, fmt.Errorf(errUnexpectedType, p.Source, "tasks.Task")
	}

	f := ts.ErrorMessage
	if f != nil {

		return *f, nil

	} else {
		return nil, nil
	}

}
func Task__RetrievalTask__resolve(p graphql.ResolveParams) (interface{}, error) {
	ts, ok := p.Source.(*tasks.Task)
	if !ok {
		return nil, fmt.Errorf(errUnexpectedType, p.Source, "tasks.Task")
	}

	f := ts.RetrievalTask
	if f != nil {

		return f, nil

	} else {
		return nil, nil
	}

}
func Task__StorageTask__resolve(p graphql.ResolveParams) (interface{}, error) {
	ts, ok := p.Source.(*tasks.Task)
	if !ok {
		return nil, fmt.Errorf(errUnexpectedType, p.Source, "tasks.Task")
	}

	f := ts.StorageTask
	if f != nil {

		return f, nil

	} else {
		return nil, nil
	}

}

var Task__type = graphql.NewObject(graphql.ObjectConfig{
	Name: "Task",
	Fields: graphql.Fields{
		"UUID": &graphql.Field{

			Type: graphql.NewNonNull(graphql.String),

			Resolve: Task__UUID__resolve,
		},
		"Status": &graphql.Field{

			Type: graphql.NewNonNull(graphql.Int),

			Resolve: Task__Status__resolve,
		},
		"WorkedBy": &graphql.Field{

			Type: graphql.String,

			Resolve: Task__WorkedBy__resolve,
		},
		"Stage": &graphql.Field{

			Type: graphql.NewNonNull(graphql.String),

			Resolve: Task__Stage__resolve,
		},
		"CurrentStageDetails": &graphql.Field{

			Type: StageDetails__type,

			Resolve: Task__CurrentStageDetails__resolve,
		},
		"PastStageDetails": &graphql.Field{

			Type: List_StageDetails__type,

			Resolve: Task__PastStageDetails__resolve,
		},
		"StartedAt": &graphql.Field{

			Type: graphql.Int,

			Resolve: Task__StartedAt__resolve,
		},
		"RunCount": &graphql.Field{

			Type: graphql.NewNonNull(graphql.Int),

			Resolve: Task__RunCount__resolve,
		},
		"ErrorMessage": &graphql.Field{

			Type: graphql.String,

			Resolve: Task__ErrorMessage__resolve,
		},
		"RetrievalTask": &graphql.Field{

			Type: RetrievalTask__type,

			Resolve: Task__RetrievalTask__resolve,
		},
		"StorageTask": &graphql.Field{

			Type: StorageTask__type,

			Resolve: Task__StorageTask__resolve,
		},
	},
})
var Tasks__type = graphql.NewObject(graphql.ObjectConfig{
	Name: "Tasks",
	Fields: graphql.Fields{
		"At": &graphql.Field{
			Type: Task__type,
			Args: graphql.FieldConfigArgument{
				"key": &graphql.ArgumentConfig{
					Type: graphql.NewNonNull(graphql.Int),
				},
			},
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				ts, ok := p.Source.(*tasks.Tasks)
				if !ok {
					return nil, errWrongKindRes
				}

				arg := p.Args["key"]
				var out *tasks.Task
				var err error
				switch ta := arg.(type) {
				// todo: key is graphql.Int, how to arrive here?
				//case ipld.Node:
				//    out, err = ts.LookupByNode(ta)
				case int64:
					l := int64(len(*ts))
					if ta >= l {
						err = fmt.Errorf("out of range")
					}
					out = &((*ts)[ta])
				default:
					return nil, fmt.Errorf("unknown key type: %T", arg)
				}

				return out, err

			},
		},
		"All": &graphql.Field{
			Type: graphql.NewList(Task__type),
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				ts, ok := p.Source.(*tasks.Tasks)
				if !ok {
					return nil, errWrongKindRes
				}
				//it := ts.ListIterator()
				children := make([]*tasks.Task, 0)
				for _, tsk := range *ts {
					children = append(children, &tsk)
				}
				//for !it.Done() {
				//    _, node, err := it.Next()
				//    if err != nil {
				//        return nil, err
				//    }
				//
				//    children = append(children, node)
				//}
				return children, nil
			},
		},
		// todo: where are Args["skip"] and Args["take"] called
		//"Range": &graphql.Field{
		//    Type: graphql.NewList(Task__type),
		//    Args: graphql.FieldConfigArgument{
		//        "skip": &graphql.ArgumentConfig{
		//            Type: graphql.NewNonNull(graphql.Int),
		//        },
		//        "take": &graphql.ArgumentConfig{
		//            Type: graphql.NewNonNull(graphql.Int),
		//        },
		//    },
		//    Resolve: func(p graphql.ResolveParams) (interface{}, error) {
		//        ts, ok := p.Source.(tasks.Tasks)
		//        if !ok {
		//            return nil, errNotNode
		//        }
		//        it := ts.ListIterator()
		//        children := make([]ipld.Node, 0)
		//
		//        for !it.Done() {
		//            _, node, err := it.Next()
		//            if err != nil {
		//                return nil, err
		//            }
		//
		//            children = append(children, node)
		//        }
		//        return children, nil
		//    },
		//},
		"Count": &graphql.Field{
			Type: graphql.NewNonNull(graphql.Int),
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				ts, ok := p.Source.(*tasks.Tasks)
				if !ok {
					return nil, errWrongKindRes
				}
				return int64(len(*ts)), nil
			},
		},
	},
})

func UpdateTask__Status__resolve(p graphql.ResolveParams) (interface{}, error) {
	ts, ok := p.Source.(*tasks.UpdateTask)
	if !ok {
		return nil, fmt.Errorf(errUnexpectedType, p.Source, "tasks.UpdateTask")
	}

	return int64(ts.Status), nil

}
func UpdateTask__ErrorMessage__resolve(p graphql.ResolveParams) (interface{}, error) {
	ts, ok := p.Source.(*tasks.UpdateTask)
	if !ok {
		return nil, fmt.Errorf(errUnexpectedType, p.Source, "tasks.UpdateTask")
	}

	f := ts.ErrorMessage
	if f != nil {

		return *f, nil

	} else {
		return nil, nil
	}

}
func UpdateTask__Stage__resolve(p graphql.ResolveParams) (interface{}, error) {
	ts, ok := p.Source.(*tasks.UpdateTask)
	if !ok {
		return nil, fmt.Errorf(errUnexpectedType, p.Source, "tasks.UpdateTask")
	}

	f := ts.Stage
	if f != nil {

		return *f, nil

	} else {
		return nil, nil
	}

}
func UpdateTask__CurrentStageDetails__resolve(p graphql.ResolveParams) (interface{}, error) {
	ts, ok := p.Source.(*tasks.UpdateTask)
	if !ok {
		return nil, fmt.Errorf(errUnexpectedType, p.Source, "tasks.UpdateTask")
	}

	f := ts.CurrentStageDetails
	if f != nil {

		return f, nil

	} else {
		return nil, nil
	}

}
func UpdateTask__WorkedBy__resolve(p graphql.ResolveParams) (interface{}, error) {
	ts, ok := p.Source.(*tasks.UpdateTask)
	if !ok {
		return nil, fmt.Errorf(errUnexpectedType, p.Source, "tasks.UpdateTask")
	}

	return ts.WorkedBy, nil

}
func UpdateTask__RunCount__resolve(p graphql.ResolveParams) (interface{}, error) {
	ts, ok := p.Source.(*tasks.UpdateTask)
	if !ok {
		return nil, fmt.Errorf(errUnexpectedType, p.Source, "tasks.UpdateTask")
	}

	return int64(ts.RunCount), nil

}

var UpdateTask__type = graphql.NewObject(graphql.ObjectConfig{
	Name: "UpdateTask",
	Fields: graphql.Fields{
		"Status": &graphql.Field{

			Type: graphql.NewNonNull(graphql.Int),

			Resolve: UpdateTask__Status__resolve,
		},
		"ErrorMessage": &graphql.Field{

			Type: graphql.String,

			Resolve: UpdateTask__ErrorMessage__resolve,
		},
		"Stage": &graphql.Field{

			Type: graphql.String,

			Resolve: UpdateTask__Stage__resolve,
		},
		"CurrentStageDetails": &graphql.Field{

			Type: StageDetails__type,

			Resolve: UpdateTask__CurrentStageDetails__resolve,
		},
		"WorkedBy": &graphql.Field{

			Type: graphql.NewNonNull(graphql.String),

			Resolve: UpdateTask__WorkedBy__resolve,
		},
		"RunCount": &graphql.Field{

			Type: graphql.NewNonNull(graphql.Int),

			Resolve: UpdateTask__RunCount__resolve,
		},
	},
})

//func init() {
//
//
//
//
//
//
//union__Any__Map.AddFieldConfig("", &graphql.Field{
//	Type: Map__type,
//	Resolve: func(p graphql.ResolveParams) (interface{}, error) {
//		ts, ok := p.Source.(tasks.Map)
//		if !ok {
//			return nil, errNotNode
//		}
//		mi := ts.MapIterator()
//		items := make(map[string]interface{})
//		for !mi.Done() {
//			k, v, err := mi.Next()
//			if err != nil {
//				return nil, err
//			}
//			// TODO: key type may not be string.
//			ks, err := k.AsString()
//			if err != nil {
//				return nil, err
//			}
//			items[ks] = v
//		}
//		return items, nil
//	},
//})
//
//
//union__Any__List.AddFieldConfig("", &graphql.Field{
//	Type: List__type,
//	Resolve: func(p graphql.ResolveParams) (interface{}, error) {
//		ts, ok := p.Source.(tasks.List)
//		if !ok {
//			return nil, errNotNode
//		}
//		li := ts.ListIterator()
//		items := make([]ipld.Node, 0)
//		for !li.Done() {
//			_, v, err := li.Next()
//			if err != nil {
//				return nil, err
//			}
//			items = append(items, v)
//		}
//		return items, nil
//	},
//})
//
//
//
//}
