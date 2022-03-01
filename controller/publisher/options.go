package publisher

import (
	"github.com/ipfs/go-ipfs/core/bootstrap"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/multiformats/go-multiaddr"
)

type (
	// Option represents a configurable parameter in a publisher.
	Option func(*options) error

	options struct {
		h         host.Host
		btstrpCfg *bootstrap.BootstrapConfig
		topic     string
		extAddrs  []multiaddr.Multiaddr
	}
)

func apply(o ...Option) (*options, error) {
	opts := &options{
		topic: "/pando/v0.0.1",
	}
	for _, apply := range o {
		if err := apply(opts); err != nil {
			return nil, err
		}
	}
	if opts.h == nil {
		var err error
		opts.h, err = libp2p.New()
		if err != nil {
			return nil, err
		}
	}
	if len(opts.extAddrs) == 0 {
		opts.extAddrs = opts.h.Addrs()
	}
	return opts, nil
}

// WithBootstrapPeers optionally sets the list of peers to which to remain connected.
// If unset, no bootstrapping will be performed.
//
// See: bootstrap.DefaultBootstrapConfig
func WithBootstrapPeers(b ...peer.AddrInfo) Option {
	return func(o *options) error {
		addrCount := len(b)
		if addrCount > 0 {
			o.btstrpCfg = &bootstrap.BootstrapConfig{
				MinPeerThreshold:  addrCount,
				Period:            bootstrap.DefaultBootstrapConfig.Period,
				ConnectionTimeout: bootstrap.DefaultBootstrapConfig.ConnectionTimeout,
				BootstrapPeers:    func() []peer.AddrInfo { return b },
			}
		}
		return nil
	}
}

// WithHost sets the host on which the publisher is exposed.
// If unset, a default libp2p host is created with random identity and listen addrtesses.
//
// See: libp2p.New.
func WithHost(h host.Host) Option {
	return func(o *options) error {
		o.h = h
		return nil
	}
}

// WithTopic sets the gossipsub topic to which announcements are made.
// Defaults to `/pando/v0.0.1` if unset.
func WithTopic(topic string) Option {
	return func(o *options) error {
		o.topic = topic
		return nil
	}
}

// WithExternalAddrs sets the addresses to include in published legs messages as the address on which
// the hsot can be reached.
// Defaults to host listen addresses.
func WithExternalAddrs(addrs []multiaddr.Multiaddr) Option {
	return func(o *options) error {
		o.extAddrs = addrs
		return nil
	}
}
