module github.com/filecoin-project/dealbot

go 1.16

require (
	github.com/benbjohnson/clock v1.1.0
	github.com/c2h5oh/datasize v0.0.0-20200825124411-48ed595a09d2
	github.com/docker/go-connections v0.4.0
	github.com/evanw/esbuild v0.12.9
	github.com/filecoin-project/go-address v0.0.6
	github.com/filecoin-project/go-amt-ipld/v2 v2.1.1-0.20201006184820-924ee87a1349 // indirect
	github.com/filecoin-project/go-fil-markets v1.13.3
	github.com/filecoin-project/go-jsonrpc v0.1.5
	github.com/filecoin-project/go-state-types v0.1.1-0.20210915140513-d354ccf10379
	github.com/filecoin-project/lotus v1.13.1
	github.com/golang-migrate/migrate/v4 v4.14.2-0.20210511063805-2e7358e012a6
	github.com/golang/mock v1.6.0
	github.com/google/uuid v1.3.0
	github.com/gorilla/handlers v1.4.2
	github.com/gorilla/mux v1.7.4
	github.com/graphql-go/graphql v0.7.9
	github.com/ipfs/go-cid v0.1.0
	github.com/ipfs/go-log/v2 v2.3.0
	github.com/ipld/go-car v0.3.2-0.20211001225732-32d0d9933823
	github.com/ipld/go-car/v2 v2.1.1-0.20211130182159-c35591a559b5
	github.com/ipld/go-ipld-graphql v0.0.0-20210608181858-5e7994523c5a
	github.com/ipld/go-ipld-prime v0.14.0
	github.com/lib/pq v1.10.3
	github.com/libp2p/go-libp2p v0.15.0
	github.com/libp2p/go-libp2p-core v0.9.0
	github.com/libp2p/go-libp2p-crypto v0.1.0
	github.com/mitchellh/go-homedir v1.1.0
	github.com/multiformats/go-multiaddr v0.4.1
	github.com/multiformats/go-multicodec v0.3.1-0.20210902112759-1539a079fd61
	github.com/polydawn/refmt v0.0.0-20201211092308-30ac6d18308e
	github.com/prometheus/client_golang v1.11.0
	github.com/robfig/cron/v3 v3.0.1
	github.com/stretchr/testify v1.7.0
	github.com/testcontainers/testcontainers-go v0.11.1
	github.com/urfave/cli/v2 v2.3.0
	github.com/willscott/ipld-dumpjson v0.0.0-20210625231845-eb416d94fa54
	golang.org/x/net v0.0.0-20210917221730-978cfadd31cf
	helm.sh/helm/v3 v3.6.2
	k8s.io/api v0.21.2
	k8s.io/cli-runtime v0.21.2
	k8s.io/client-go v0.21.2
	rsc.io/letsencrypt v0.0.3 // indirect
	sigs.k8s.io/yaml v1.2.0
)

replace github.com/filecoin-project/filecoin-ffi => github.com/filecoin-project/ffi-stub v0.2.0
