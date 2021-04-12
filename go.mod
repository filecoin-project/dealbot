module github.com/filecoin-project/dealbot

go 1.16

require (
	github.com/BurntSushi/toml v0.3.1
	github.com/c2h5oh/datasize v0.0.0-20200825124411-48ed595a09d2
	github.com/filecoin-project/go-address v0.0.5
	github.com/filecoin-project/go-fil-markets v1.2.4
	github.com/filecoin-project/go-jsonrpc v0.1.4-0.20210217175800-45ea43ac2bec
	github.com/filecoin-project/go-state-types v0.1.0
	github.com/filecoin-project/lotus v1.5.4-0.20210402110700-eede19fb0b2d
	github.com/google/uuid v1.1.2
	github.com/gorilla/mux v1.7.4
	github.com/ipfs/go-cid v0.0.7
	github.com/ipfs/go-log/v2 v2.1.2
	github.com/lib/pq v1.9.0
	github.com/mitchellh/go-homedir v1.1.0
	github.com/multiformats/go-multiaddr v0.3.1
	github.com/urfave/cli/v2 v2.3.0
	github.com/whyrusleeping/cbor-gen v0.0.0-20210303213153-67a261a1d291 // indirect
	golang.org/x/xerrors v0.0.0-20200804184101-5ec99f83aff1
)

replace github.com/filecoin-project/filecoin-ffi => github.com/filecoin-project/ffi-stub v0.1.0
