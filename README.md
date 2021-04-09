# Dealbot

A tool to test and analyze storage and retrieval deal capability on the Filecoin network.

## Getting Started

Clone the repo and build the dependencies:

```console
$ git clone git@github.com:filecoin-project/dealbot.git
$ cd dealbot
```

Build the `dealbot` binary to the root of the project directory:

```console
$ go mod tidy
$ go build
```
Dealbot requires a local synced instance of [Lotus](https://github.com/filecoin-project/lotus/) to communicate with miners and the chain. This can be a Devnet instance for testing or a node connected to a larger network. The node needs to have wallet address with funds for making deals and a data cap for making verified deals (if verified deals are necessary).

### Usage

Dealbot runs on multiple machines with a centralized controller. The controller can be started with:

```
./dealbot controller --configpath config.toml
```

See dealbot-example.toml for configuration parameters. Individual Dealbot nodes run with the daemon command:
 
```
./dealbot --api [LOTUS_API_URL] controller --configpath config.toml
```

The api parameter points to the Lotus api and can be specified as a url token pair or by specifying the lotus path either as a parameter or environment variable:

```
--api [lotus_api_url]:[lotus_api_token]
export FULLNODE_API_INFO=[lotus_api_url]:[lotus_api_token]
--lotus-path ~/.lotus
export LOTUS_PATH=~/.lotus
```

Dealbot can also run individual storage or retrieval task with:

```
./dealbot --api [api] storage-deal --deata-dir [shared-dir] --miner [miner-address] --size 2GB 
``` 

or 

```
./dealbot --api [api] retrieval-deal --deata-dir [shared-dir] --miner [miner-address] --cid [payload-cid] 
``` 

## Versioning and Releases

TBD

## Code of Conduct

Dealbot follows the [Filecoin Project Code of Conduct](https://github.com/filecoin-project/community/blob/master/CODE_OF_CONDUCT.md). Before contributing, please acquaint yourself with our social courtesies and expectations.


## Contributing

Welcoming [new issues](https://github.com/filecoin-project/dealbot/issues/new) and [pull requests](https://github.com/filecoin-project/dealbot/pulls).


## License

The Filecoin Project and Dealbot is dual-licensed under Apache 2.0 and MIT terms:

- Apache License, Version 2.0, ([LICENSE-APACHE](https://github.com/filecoin-project/dealbot/blob/master/LICENSE-APACHE) or http://www.apache.org/licenses/LICENSE-2.0)
- MIT license ([LICENSE-MIT](https://github.com/filecoin-project/dealbot/blob/master/LICENSE-MIT) or http://opensource.org/licenses/MIT)