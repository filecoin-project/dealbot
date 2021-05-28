module github.com/filecoin-project/dealbot

go 1.16

require (
	github.com/c2h5oh/datasize v0.0.0-20200825124411-48ed595a09d2
	github.com/filecoin-project/go-address v0.0.5
	github.com/filecoin-project/go-fil-markets v1.2.5
	github.com/filecoin-project/go-jsonrpc v0.1.4-0.20210217175800-45ea43ac2bec
	github.com/filecoin-project/go-state-types v0.1.0
	github.com/filecoin-project/lotus v1.9.0
	github.com/golang-migrate/migrate/v4 v4.14.2-0.20210511063805-2e7358e012a6
	github.com/google/uuid v1.1.2
	github.com/gorilla/handlers v1.4.2
	github.com/gorilla/mux v1.7.4
	github.com/graphql-go/graphql v0.7.9
	github.com/ipfs/go-block-format v0.0.3
	github.com/ipfs/go-cid v0.0.7
	github.com/ipfs/go-log/v2 v2.1.2
	github.com/ipld/go-car v0.1.1-0.20201119040415-11b6074b6d4d
	github.com/ipld/go-ipld-graphql v0.0.0-20210425153336-bd6fb64874b6
	github.com/ipld/go-ipld-prime v0.7.1-0.20210519202903-3a67953d6ef3
	github.com/lib/pq v1.9.0
	github.com/libp2p/go-libp2p-core v0.7.0
	github.com/libp2p/go-libp2p-crypto v0.1.0
	github.com/mitchellh/go-homedir v1.1.0
	github.com/multiformats/go-multiaddr v0.3.1
	github.com/multiformats/go-multicodec v0.2.0
	github.com/prometheus/client_golang v1.6.0
	github.com/robfig/cron/v3 v3.0.1
	github.com/stretchr/testify v1.7.0
	github.com/urfave/cli/v2 v2.3.0
	modernc.org/sqlite v1.10.6
)

replace github.com/filecoin-project/filecoin-ffi => github.com/filecoin-project/ffi-stub v0.1.0
