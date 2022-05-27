# Dealbot

A tool to test and analyze storage and retrieval deal capability on the Filecoin network.

## Getting Started

Clone the repo and build:

	git clone git@github.com:filecoin-project/dealbot.git
	cd dealbot
	go build

Dealbot requires a local synced instance of [Lotus](https://github.com/filecoin-project/lotus/) to communicate with miners and the chain. This can be a Devnet instance for testing or a node connected to a larger network. The node needs to have wallet address with funds for making deals and a data cap for making verified deals (if verified deals are necessary).

### Usage

Dealbot runs on multiple machines with a centralized controller. The controller can be started with:

	./dealbot controller --configpath config.toml

See dealbot-example.toml for configuration parameters. Individual Dealbot nodes run with the daemon command:

	./dealbot --api [LOTUS_API_URL] daemon --configpath config.toml

The `--api` parameter points to the Lotus API and can be specified as a URL token pair. Alternatively you can specify `--lotus-path` either as a parameter or environment variable:

	--api [lotus_api_url]:[lotus_api_token]
	export FULLNODE_API_INFO=[lotus_api_url]:[lotus_api_token]
	--lotus-path ~/.lotus
	export LOTUS_PATH=~/.lotus

Dealbot can also run individual storage or retrieval task when invoked from the command-line with:

	./dealbot --api [api] storage-deal --data-dir [shared-dir] --miner [miner-address] --size 2GB

or

	./dealbot --api [api] retrieval-deal --data-dir [shared-dir] --miner [miner-address] --cid [payload-cid]

To start Lotus locally, or tunnel to a remote Lotus, see [devnet/README.md](devnet/README.md).

### Flags

Dealbot Controller

| Flag | Env Var | Function |
| :--- | :--- | --- |
| listen | `DEALBOT_LISTEN` | exposed `host:port` for daemons to contact and for tasking the system |
| metrics | `DEALBOT_METRICS` | either `prometheus` to expose a `/metrics` api, or `log` to write metrics to stdout |
| identity | `DEALBOT_IDENTITY_KEYPAIR` | filepath of a libp2p identity to sign public records of dealbot activity |
| driver | `DEALBOT_PERSISTENCE_DRIVER` | `postgres` |
| dbloc | `DEALBOT_PERSISTENCE_CONN` |  db conn string from postgres |
| devAssetDir| - | serve controller assets from disk rather the compiled binary for development |
| basicauth | `DEALBOT_BASICAUTH` | basic authentication credentials if the controller is being served behind them to make xhrs work in that environment |
| datapointlog | `DEALBOT_DATAPOINT_LOG` | file / stream to write out a json line for each completed task |
| gateway-api | `DEALBOT_LOTUS_GATEWAY` | address of lotus gateway to query for wallet balances for controller UX |

Dealbot Daemon

| Flag | Env Var | Function |
| :--- | :--- | --- |
|  id | `DEALBOT_ID` | The worker name to report to the controller |
| listen | `DEALBOT_LISTEN` | a `host:port` to bind to when metrics are exposed |
| stage-timeout | `STAGE_TIMEOUT` | a list of stagenames and timeouts (example: DealAccepted=15m) |
| tags | `DEALBOT_TAGS` | tags to use when accepting tasks |
| workers | `DEALBOT_WORKERS` | how many tasks to accept at a time |
| minfil | `DEALBOT_MIN_FIL` | minimum balance lotus must report before the bot will accept tasks |
| mincap | `DEALBOT_MIN_CAP` | minimum dealcap lotus must report before the bot will accept tasks |
| posthook | `DEALBOT_POST_HOOK` | a bash script that will be run as `bash $posthook $uuid` when a task finishes |
| endpoint | `DEALBOT_CONTROLLER_ENDPOINT` | the `host:port` of the controller to ask for tasks |
| data-dir | `DEALBOT_DATA_DIRECTORY` | the directory for the bot to make cars in or verify they have shown up in |
| node-data-dir | `DEALBOT_NODE_DATA_DIRECTORY` | the directory for lotus to import the cars from or write them to |
| wallet | `DEALBOT_WALLET_ADDRESS` | an explicit wallet to use with lotus if not the default one |
 
## Versioning and Releases

Tagged releases indicate versions run on our local deployment.
Semver is used to indicate when data would be lost on downgrading (miner version bumps) and when data becomes incompatible (major version bumps)

## Code of Conduct

Dealbot follows the [Filecoin Project Code of Conduct](https://github.com/filecoin-project/community/blob/master/CODE_OF_CONDUCT.md). Before contributing, please acquaint yourself with our social courtesies and expectations.


## Contributing

We welcome [new issues](https://github.com/filecoin-project/dealbot/issues/new) and [pull requests](https://github.com/filecoin-project/dealbot/pulls).


## License

The Filecoin Project and Dealbot is dual-licensed under Apache 2.0 and MIT terms:

- Apache License, Version 2.0, ([LICENSE-APACHE](https://github.com/filecoin-project/dealbot/blob/master/LICENSE-APACHE) or http://www.apache.org/licenses/LICENSE-2.0)
- MIT license ([LICENSE-MIT](https://github.com/filecoin-project/dealbot/blob/master/LICENSE-MIT) or http://opensource.org/licenses/MIT)
