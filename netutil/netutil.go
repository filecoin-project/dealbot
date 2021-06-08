package netutil

import (
	"io"
	"strings"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/multiformats/go-multiaddr"
	"golang.org/x/net/context"
)

func TryAcquireLatency(ctx context.Context, ai peer.AddrInfo) (multiaddr.Multiaddr, int64, error) {
	cctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	h, err := libp2p.New(cctx)
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

	scheduledChan := make(chan struct{}, 1)
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		p := []byte{0x00}
		scheduledChan <- struct{}{}
		_, serr := stream.Read(p)
		if serr == io.EOF || serr == io.ErrUnexpectedEOF || strings.Contains(serr.Error(), "connection reset by peer") {
			end = time.Now()
		} else {
			err = serr
		}
	}()

	// trigger closing, which should cause the stream read to become eof error out.
	<-scheduledChan
	stream.Close()
	conn.Close()
	wg.Wait()

	return remote, end.Sub(start).Milliseconds(), err
}
