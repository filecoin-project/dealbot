package publisher

import (
	"bytes"
	"context"
	"errors"
	"io"
	"sync"

	"github.com/filecoin-project/go-legs"
	"github.com/filecoin-project/go-legs/dtsync"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	dssync "github.com/ipfs/go-datastore/sync"
	"github.com/ipfs/go-ipfs/core/bootstrap"
	logging "github.com/ipfs/go-log/v2"
	"github.com/ipld/go-ipld-prime"
	"github.com/ipld/go-ipld-prime/linking"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	basicnode "github.com/ipld/go-ipld-prime/node/basic"
	"github.com/ipld/go-ipld-prime/storage"
	pando "github.com/kenlabs/pando/pkg/types/schema"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
)

// PandoTopic is the gossipsub topic on which the metadata announcements are published.
const PandoTopic = "/pando/v0.0.1"

var (
	log = logging.Logger("publisher")

	_ Publisher = (*PandoPublisher)(nil)

	errAlreadyStarted  = errors.New("already started")
	errNotStarted      = errors.New("not started")
	latestPublishedCid = datastore.NewKey("/latest")
)

type PandoPublisher struct {
	opts *options
	h    host.Host
	ls   ipld.LinkSystem
	ds   datastore.Batching

	pub legs.Publisher
	// lock synchronizes calls to all Publisher APIs.
	lock   sync.Mutex
	closer io.Closer
	store  storage.ReadableStorage
}

// NewPandoPublisher instantiates a new publisher that publishes announcements compatible with the
// Pando service by wrapping the CID of the original data within a Pando's metadata instance.
//
// A user may call PandoPublisher.Publish with the CID of original data to produce one such
// announcement. Note that any CID passed to PandoPublisher.Publish must be available to read via
// the given store. See: PandoPublisher.Publish.
//
// This publisher must be started via PandoPublisher.Start prior to use and shut down when no longer
// needed via PandoPublisher.Shutdown.
//
// The ds datastore is used internally by the publisher and is not closed upon shutdown.
func NewPandoPublisher(ds datastore.Batching, store storage.ReadableStorage, o ...Option) (Publisher, error) {
	opts, err := apply(o...)
	if err != nil {
		return nil, err
	}

	p := &PandoPublisher{
		opts:  opts,
		h:     opts.h,
		ds:    ds,
		store: store,
		ls:    cidlink.DefaultLinkSystem(),
	}
	p.ls.StorageReadOpener = p.storageReadOpener
	p.ls.StorageWriteOpener = p.storageWriteOpener

	log.Infow("Instantiated pando publisher", "peerID", p.id(), "listenAdds", p.h.Addrs())
	return p, nil
}

func (p *PandoPublisher) Start(ctx context.Context) (err error) {
	// Dealbot registration as a provider on pando side is done manually.
	// TODO: If the registration is idempotent, maybe re-register here automatically?
	p.lock.Lock()
	defer p.lock.Unlock()
	log := log.With("host", p.id())

	if p.isStarted() {
		log.Error("Already started; cannot start again.")
		return errAlreadyStarted
	}

	if p.opts.btstrpCfg != nil {
		bLog := log.With("peers", p.opts.btstrpCfg.BootstrapPeers)
		closer, err := bootstrap.Bootstrap(p.id(), p.h, nil, *p.opts.btstrpCfg)
		if err != nil {
			bLog.Errorw("Failed to bootstrap with peers", "err", err)
			return err
		}
		bLog.Info("Bootstrapped with peers successfully")
		p.closer = closer
	}

	// Use ephemeral storage to avoid UTF-8 encoding issues.
	//
	// Note that the datastore here is only used by datatrasnfer. The datatransfer manager uses the
	// datastore to store byte value of CIDs, i.e. cid.Byte(). Since the backing db table uses type
	// text as the data column, and go postgres requires UTF-8 encoding we will get an error when
	// the SQL driver attempts to encode bytes, e.g.: pq: invalid byte sequence for encoding "UTF8".
	//
	// For now use an ephemeral storage for the datatransfer.
	//TODO: Understand if we need a persistent datastore for datatransfer at all here.
	//      If so, wrap a datastore with customized byte encoding so that postgres is happy.
	lds := dssync.MutexWrap(datastore.NewMapDatastore())
	p.pub, err = dtsync.NewPublisher(p.h, lds, p.ls, PandoTopic)
	if err != nil {
		log.Errorw("Failed to initialize legs publisher", "err", err)
		return err
	}

	latest, err := p.getLatest(ctx)
	if err != nil {
		log.Errorw("Failed to get latest while starting pando publisher", "err", err)
		return err
	}
	log = log.With("head", latest)

	if latest != cid.Undef {
		if err := p.pub.SetRoot(ctx, latest); err != nil {
			log.Errorw("Failed to update the head CID while starting pando publisher", "err", err)
			return err
		}
	}

	log.Infow("Started pando publisher", "extAddrs", p.opts.extAddrs)
	return nil
}

func (p *PandoPublisher) isStarted() bool {
	// Use p.pub as flag to check if publisher is already started.
	return p.pub != nil
}

// Publish wraps the given CID into a Pando metadata instance and announces the CID of resulting
// metadata.
//
// The metadata produced, simply uses the byte value of the given CID as payload of the Pando
// metadata schema. See: https://github.com/kenlabs/pando
//
// Note that the given CID must be preset in the store passed to the publisher at
// initialization.
func (p *PandoPublisher) Publish(ctx context.Context, c cid.Cid) error {
	p.lock.Lock()
	defer p.lock.Unlock()

	if !p.isStarted() {
		return errNotStarted
	}

	log := log.With("payload", c)
	// Construct optional link to the previously published metadata.
	previous, err := p.getLatest(ctx)
	if err != nil {
		log.Errorw("Failed to get the latest metadata link", "err", err)
		return err
	}
	var previousLink ipld.Link
	if previous != cid.Undef {
		log = log.With("previous", previous)
		previousLink = cidlink.Link{Cid: previous}
	}

	// According to the Pando metadata schema, the payload can be of type Any. Here we use the CID
	// link itself as the metadata payload. The link itself is then traversable via the graphsync
	// server exposed by this publisher, which allows Pando to fully traverse and store the dealbot
	// DAG to which the link points.
	payload := basicnode.NewLink(cidlink.Link{Cid: c})
	pm, err := pando.NewMetaWithPayloadNode(payload, p.id(), p.key(), previousLink)
	if err != nil {
		log.Errorw("Failed to instantiate pando metadata", "err", err)
		return err
	}

	// Store the metadata in linksystem so that sync requests are traversable
	node, err := pm.ToNode()
	if err != nil {
		log.Errorw("Failed to convert pando metadata to IPLD node", "err", err)
		return err
	}
	pmLnk, err := p.ls.Store(linking.LinkContext{Ctx: ctx}, pando.LinkProto, node)
	if err != nil {
		log.Errorw("Failed to store pando metadata in linksystem", "err", err)
		return err
	}
	latest := pmLnk.(cidlink.Link).Cid
	log = log.With("latest", latest)

	// Update internal reference to the latest
	if err := p.setLatest(ctx, latest); err != nil {
		log.Errorw("Failed to update reference to the latest metadata", "err", err)
		return err
	}

	// Announce the latest
	if err := p.pub.UpdateRootWithAddrs(ctx, latest, p.opts.extAddrs); err != nil {
		log.Errorw("Failed to update the latest legs root", "err", err)
		return err
	}

	log.Infow("Published the latest root successfully", "extAddrs", p.opts.extAddrs)
	return nil
}

func (p *PandoPublisher) id() peer.ID {
	return p.h.ID()
}

func (p *PandoPublisher) key() crypto.PrivKey {
	return p.h.Peerstore().PrivKey(p.id())
}

func (p *PandoPublisher) getLatest(ctx context.Context) (cid.Cid, error) {
	d, err := p.ds.Get(ctx, latestPublishedCid)
	if err == datastore.ErrNotFound {
		return cid.Undef, nil
	}
	if err != nil {
		return cid.Undef, err
	}
	return cid.Decode(string(d))
}

func (p *PandoPublisher) setLatest(ctx context.Context, c cid.Cid) error {
	return p.ds.Put(ctx, latestPublishedCid, []byte(c.String()))
}

func (p *PandoPublisher) storageReadOpener(lc linking.LinkContext, l ipld.Link) (io.Reader, error) {
	// Use the link prototype as a hint to only check datastore if link prototype matches pando metadata.
	// Because, that datastore only stores pando metadata.
	if l.Prototype() == pando.LinkProto {
		v, err := p.ds.Get(lc.Ctx, dsKey(l))
		if err == nil {
			return bytes.NewBuffer(v), nil
		}
		// If not found, look in p.store anyway.
		if err != nil && err != datastore.ErrNotFound {
			return nil, err
		}
	}

	// Look in p.store since we are probably dealing with a link encoded as pando metadata payload,
	// which is of type RecordUpdate and is stored in the db store.
	return storage.GetStream(lc.Ctx, p.store, l.String())
}

func (p *PandoPublisher) storageWriteOpener(lc linking.LinkContext) (io.Writer, linking.BlockWriteCommitter, error) {
	buf := bytes.NewBuffer(nil)
	return buf, func(l ipld.Link) error {
		return p.ds.Put(lc.Ctx, dsKey(l), buf.Bytes())
	}, nil
}

func dsKey(l ipld.Link) datastore.Key {
	return datastore.NewKey(l.String())
}

// Shutdown shuts down this publisher.
// Once shut down, this publisher should be discarded and can no longer be used.
func (p *PandoPublisher) Shutdown(_ context.Context) error {
	p.lock.Lock()
	defer p.lock.Unlock()

	if !p.isStarted() {
		return errNotStarted
	}

	// Close all resources.
	var cerr error
	if p.closer != nil {
		cerr = p.closer.Close()
	}
	perr := p.pub.Close()

	// Return the first non-nil error
	if cerr == nil {
		return perr
	}
	return cerr
}
