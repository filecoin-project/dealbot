package netutil

import (
	"io"
	"strings"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p/config"
	"github.com/multiformats/go-multiaddr"
	"golang.org/x/net/context"
)

func TryAcquireLatency(ctx context.Context, ai peer.AddrInfo, makeHost func(ctx context.Context, opts ...config.Option) (host.Host, error)) (multiaddr.Multiaddr, int64, error) {
	cctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	h, err := makeHost(cctx)
	if err != nil {
		return nil, 0, err
	}

	h.Peerstore().AddAddrs(ai.ID, ai.Addrs, time.Hour)
	conn, err := h.Network().DialPeer(cctx, ai.ID)
	if err != nil {
		return nil, 0, err
	}
	remote := conn.RemoteMultiaddr()
	start := time.Now()
	var end time.Time
	stream, err := conn.NewStream()
	stream.SetReadDeadline(time.Now().Add(3 * time.Second))

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		p := []byte{0x00}
		_, serr := stream.Read(p)
		if serr == io.EOF || serr == io.ErrUnexpectedEOF || serr == nil || strings.Contains(serr.Error(), "connection reset by peer") {
			end = time.Now()
		} else {
			err = serr
		}
	}()

	// send a write of random bytes, which should make the other side unhappy.
	stream.Write([]byte("-latencytest-\n\x00"))
	wg.Wait()
	defer conn.Close()

	return remote, end.Sub(start).Milliseconds(), err
}
