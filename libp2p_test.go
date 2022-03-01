package main

import (
	"context"
	"fmt"
	"testing"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/stretchr/testify/require"
)

func Test_bootstraeeepHost(t *testing.T) {
	peerid := "12D3KooWMm4sgwMsbzdGnLNhQv4dgMvqyp2JAAPHJHtRWVvjG8rn"
	//addrinfo := "/dns/adbbccfa94ea64c40b59dc34ccf16219-dfb84bf0fd1db508.elb.us-east-1.amazonaws.com/tcp/8762/p2p/" + peerid
	addrinfo := "/dns/dealbot-mainnet-l.mainnet-us-east-1.filops.net/tcp/8762/p2p/" + peerid
	//addrinfo := "/ip4/184.73.21.166/tcp/40664/p2p/" + peerid
	//peerid := "12D3KooWNU48MUrPEoYh77k99RbskgftfmSm3CdkonijcM5VehS9"
	//addrinfo := "/ip4/52.14.211.248/tcp/9013/p2p/" + peerid
	fromString, err := peer.AddrInfoFromString(addrinfo)
	require.NoError(t, err)
	host, err := libp2p.New()
	require.NoError(t, err)
	err = host.Connect(context.Background(), *fromString)
	require.NoError(t, err)
	idFromString, err := peer.Decode(peerid)
	require.NoError(t, err)
	protocols, err := host.Peerstore().GetProtocols(idFromString)
	require.NoError(t, err)
	for _, protocol := range protocols {
		fmt.Println(protocol)
	}

	//decodeString, err := base64.StdEncoding.DecodeString("CAESQKkp+yk6YH0UdJ5jlZzHIKdgT2yRzDgkGSlEzVH56xTWsXPyPN0xmh2sN6ElkZ1Y94EbT/vAzcX4Qz3xD+Ik29s=")
	//require.NoError(t, err)
	//
	//key, err := crypto.UnmarshalPrivateKey(decodeString)
	//require.NoError(t, err)
	//
	//fmt.Println(key.GetPublic())
}
