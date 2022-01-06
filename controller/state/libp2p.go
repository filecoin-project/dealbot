package state

import (
	"database/sql"

	"github.com/ipfs/go-datastore"
	csms "github.com/libp2p/go-conn-security-multistream"
	crypto "github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/transport"
	mplex "github.com/libp2p/go-libp2p-mplex"
	noise "github.com/libp2p/go-libp2p-noise"
	"github.com/libp2p/go-libp2p-peerstore/pstoremem"
	swarm "github.com/libp2p/go-libp2p-swarm"
	tls "github.com/libp2p/go-libp2p-tls"
	tptu "github.com/libp2p/go-libp2p-transport-upgrader"
	yamux "github.com/libp2p/go-libp2p-yamux"
	basichost "github.com/libp2p/go-libp2p/p2p/host/basic"
	msmux "github.com/libp2p/go-stream-muxer-multistream"
	"github.com/libp2p/go-tcp-transport"
	ws "github.com/libp2p/go-ws-transport"

	sqlds "github.com/ipfs/go-ds-sql"
	pg "github.com/ipfs/go-ds-sql/postgres"
)

func NewHost(priv crypto.PrivKey) (host.Host, error) {
	ps, err := pstoremem.NewPeerstore()
	if err != nil {
		return nil, err
	}
	pub := priv.GetPublic()
	pid, err := peer.IDFromPublicKey(pub)
	if err != nil {
		return nil, err
	}

	if err := ps.AddPrivKey(pid, priv); err != nil {
		return nil, err
	}
	if err := ps.AddPubKey(pid, pub); err != nil {
		return nil, err
	}

	net, err := swarm.NewSwarm(pid, ps)
	if err != nil {
		return nil, err
	}

	host, err := basichost.NewHost(net, &basichost.HostOpts{})
	if err != nil {
		return nil, err
	}

	secMuxer := new(csms.SSMuxer)
	noiseSec, _ := noise.New(priv)
	secMuxer.AddTransport(noise.ID, noiseSec)
	tlsSec, _ := tls.New(priv)
	secMuxer.AddTransport(tls.ID, tlsSec)

	muxMuxer := msmux.NewBlankTransport()
	muxMuxer.AddTransport("/yamux/1.0.0", yamux.DefaultTransport)
	muxMuxer.AddTransport("/mplex/6.7.0", mplex.DefaultTransport)

	upgrader, err := tptu.New(secMuxer, muxMuxer)
	if err != nil {
		return nil, err
	}

	tcpT, _ := tcp.NewTCPTransport(upgrader)
	for _, t := range []transport.Transport{
		tcpT,
		ws.New(upgrader),
	} {
		if err := net.AddTransport(t); err != nil {
			return nil, err
		}
	}

	host.Start()
	return host, nil
}

func dbDS(table string, db *sql.DB) datastore.Batching {
	queries := pg.NewQueries(table)
	ds := sqlds.NewDatastore(db, queries)
	return ds
}
