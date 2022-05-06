package main

import (
	"fmt"
	"os"

	"github.com/filecoin-project/dealbot/controller/publisher"
	"github.com/filecoin-project/go-legs/dtsync"
	"github.com/filecoin-project/go-legs/p2p/protocol/head"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	dssync "github.com/ipfs/go-datastore/sync"
	"github.com/ipld/go-ipld-prime"
	"github.com/ipld/go-ipld-prime/codec/dagjson"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	basicnode "github.com/ipld/go-ipld-prime/node/basic"
	"github.com/ipld/go-ipld-prime/storage/memstore"
	"github.com/ipld/go-ipld-prime/traversal/selector"
	selectorbuilder "github.com/ipld/go-ipld-prime/traversal/selector/builder"
	pando "github.com/kenlabs/pando/pkg/types/schema"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/peerstore"
	"github.com/urfave/cli/v2"
)

const (
	defaultServerAddr = "/dns/dealbot-mainnet-libp2p.mainnet-us-east-1.filops.net/tcp/8762/p2p/12D3KooWNnK4gnNKmh6JUzRb34RqNcBahN5B8v18DsMxQ8mCqw81"
)

var (
	serverAddrIfoFlag    string
	rootCidFlag          string
	skipEventsFlag       bool
	skipRecordsChainFlag bool
	recursionLimitFlag   int64
)

func main() {
	app := &cli.App{
		Name:  "dealbot",
		Usage: "A client CLI to interact with dealbot.",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "server-addrinfo",
				Aliases:     []string{"s"},
				Usage:       "The dealbot server libp2p addrinfo.",
				Value:       defaultServerAddr,
				Destination: &serverAddrIfoFlag,
			},
		},
		Commands: []*cli.Command{
			{
				Name:        "metadata",
				Aliases:     []string{"md"},
				Description: "Shows information about the Pando metadata published by dealbot",
				Subcommands: []*cli.Command{
					{
						Name:        "head",
						Description: "Prints the CID of current head metadata DAG chain",
						Action: func(cctx *cli.Context) error {
							dest, err := peer.AddrInfoFromString(serverAddrIfoFlag)
							if err != nil {
								return err
							}
							h, err := libp2p.New()
							if err != nil {
								return err
							}

							h.Peerstore().AddAddrs(dest.ID, dest.Addrs, peerstore.TempAddrTTL)
							head, err := head.QueryRootCid(cctx.Context, h, publisher.PandoTopic, dest.ID)
							if err != nil {
								return err
							}

							if head == cid.Undef {
								_, err = fmt.Fprintln(cctx.App.ErrWriter, "No head present")
							} else {
								_, err = fmt.Fprintln(cctx.App.Writer, head.String())
							}
							return err
						},
					},
					{
						Name: "export",
						Description: `Prints the finished tasks in metadata DAG chain in DAG-JSON format starting from head CID and 
recursively traversing links. Each node in DAG is printed in a new line as a JSON object with a 
single key representing the CID of the node and its value representing the DAG-JSON encoded value of
the node.

The export can optionally resume from a specific CID if provided via 'root-cid' flag. Otherwise, the
traversal will begin from the latest head CID, automatically detected via dealbot API. Note that to
resume traversal, the given CID must point to a record chain, otherwise the traversal will not find
link to previous record and cannot proceed further.`,
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:    "root-cid",
								Aliases: []string{"r"},
								Usage: `Optionally specifies the root records chain CID from which to begin traversal. Note that the CID must
point to dealbot's records chain.`,
								DefaultText: "automatically extracted from the payload of latest published Pando metadata.",
								Destination: &rootCidFlag,
							},
							&cli.BoolFlag{
								Name:        "skip-events",
								Aliases:     []string{"se"},
								Usage:       "Whether to skip printing the events linked in finished tasks.",
								Destination: &skipEventsFlag,
							},
							&cli.BoolFlag{
								Name:        "skip-records-chain",
								Aliases:     []string{"sr"},
								Usage:       "Whether to skip printing dealbot's records chain that lists finished tasks.",
								Destination: &skipRecordsChainFlag,
							},
							&cli.Int64Flag{
								Name:        "recursion-limit",
								Aliases:     []string{"l"},
								Usage:       "The maximum recursion depth limit when traversing metadata records. Any value less than zero is treated as no limit",
								DefaultText: "-1, i.e. no limit.",
								Value:       -1,
								Destination: &recursionLimitFlag,
							},
						},
						Action: func(cctx *cli.Context) error {
							dest, err := peer.AddrInfoFromString(serverAddrIfoFlag)
							if err != nil {
								return err
							}
							h, err := libp2p.New()
							if err != nil {
								return err
							}
							h.Peerstore().AddAddrs(dest.ID, dest.Addrs, peerstore.TempAddrTTL)

							lsys := cidlink.DefaultLinkSystem()
							store := &memstore.Store{}
							lsys.SetReadStorage(store)
							lsys.SetWriteStorage(store)
							ssb := selectorbuilder.NewSelectorSpecBuilder(basicnode.Prototype.Any)
							sync, err := dtsync.NewSync(h, dssync.MutexWrap(datastore.NewMapDatastore()), lsys, func(id peer.ID, c cid.Cid) {
								if id == dest.ID {
									node, err := lsys.Load(ipld.LinkContext{Ctx: cctx.Context}, cidlink.Link{Cid: c}, basicnode.Prototype.Any)
									if err != nil {
										panic(err)
									}

									if shouldOutput(node) {
										fmt.Fprintf(cctx.App.Writer, `{"%s":`, c)
										err = dagjson.Encode(node, cctx.App.Writer)
										if err != nil {
											panic(err)
										}
										fmt.Fprintln(cctx.App.Writer, "}")
									}
								}
							})
							if err != nil {
								return err
							}
							defer sync.Close()
							syncer := sync.NewSyncer(dest.ID, publisher.PandoTopic)

							var rootCid cid.Cid
							if rootCidFlag == "" {

								// If root record CID is not specified, then:
								//  1. Find the head of published Pando metadata.
								//  2. Parse its payload as either a link or a bytes representing a link
								//  3. The resulting link in payload is the head of UpdateRecords chain.

								headMetaCid, err := head.QueryRootCid(cctx.Context, h, publisher.PandoTopic, dest.ID)
								if err != nil {
									return err
								}

								if headMetaCid == cid.Undef {
									// There is no head, meaning there is no prior publication of metadata.
									// Print an empty JSON object and we are done.
									_, err = fmt.Fprintln(cctx.App.Writer, "{}")
									return err
								}

								selHead := ssb.ExploreRecursive(selector.RecursionLimitDepth(0), ssb.ExploreAll(ssb.ExploreRecursiveEdge())).Node()
								err = syncer.Sync(cctx.Context, rootCid, selHead)
								if err != nil {
									return err
								}

								node, err := lsys.Load(ipld.LinkContext{Ctx: cctx.Context}, cidlink.Link{Cid: headMetaCid}, pando.MetadataPrototype)
								if err != nil {
									return err
								}
								md, err := pando.UnwrapMetadata(node)
								if err != nil {
									return err
								}
								switch md.Payload.Kind() {
								case ipld.Kind_Bytes:
									bytes, err := md.Payload.AsBytes()
									if err != nil {
										return err
									}

									_, rootCid, err = cid.CidFromBytes(bytes)
									if err != nil {
										return err
									}
								case ipld.Kind_Link:
									lnk, err := md.Payload.AsLink()
									if err != nil {
										return err
									}
									rootCid = lnk.(cidlink.Link).Cid
								}
							} else {
								rootCid, err = cid.Decode(rootCidFlag)
								if err != nil {
									return err
								}
							}

							if rootCid == cid.Undef {
								// There is no UpdateRecord, meaning there is no prior records.
								// Print an empty JSON object and we are done.
								_, err = fmt.Fprintln(cctx.App.Writer, "{}")
								return err
							}

							recSel := ssb.ExploreRecursive(recursionLimit(),
								ssb.ExploreUnion(
									ssb.ExploreFields(
										func(efsb selectorbuilder.ExploreFieldsSpecBuilder) {
											if !skipEventsFlag {
												// No need to traverse Events if we are not going to print them.
												efsb.Insert("Events", ssb.ExploreRecursiveEdge())
											}
											efsb.Insert("Records", ssb.ExploreAll(
												ssb.ExploreFields(
													func(efsb selectorbuilder.ExploreFieldsSpecBuilder) {
														efsb.Insert("Record", ssb.ExploreRecursiveEdge())
													})))
											efsb.Insert("Previous", ssb.ExploreRecursiveEdge())
										}),
								)).Node()

							return syncer.Sync(cctx.Context, rootCid, recSel)
						},
					},
				},
			},
		},
	}
	err := app.Run(os.Args)
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err.Error())
	}
}

func shouldOutput(node ipld.Node) bool {
	return isFinishedTaskNode(node) ||
		(isEventsNode(node) && !skipEventsFlag) ||
		(isRecordChain(node) && !skipRecordsChainFlag)
}

func isRecordChain(node ipld.Node) bool {
	return hasKey(node, "Records")
}

func isFinishedTaskNode(node ipld.Node) bool {
	return hasKey(node, "Events")
}

func isEventsNode(node ipld.Node) bool {
	return hasKey(node, "Logs")
}

func hasKey(node ipld.Node, key string) bool {
	n, err := node.LookupByString(key)
	return err == nil && n != nil
}

func recursionLimit() selector.RecursionLimit {
	if recursionLimitFlag < 0 {
		return selector.RecursionLimitNone()
	}
	return selector.RecursionLimitDepth(recursionLimitFlag)
}
