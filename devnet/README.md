# devnet

Runs a Lotus daemon and miner, ready to be used for dealbot's integration tests.

To run it locally, install ./cmd/devnet and the lotus binaries into your $PATH, and
simply run one of the integration tests - it should start devnet automatically.

### Building and installing Lotus

	git clone https://github.com/filecoin-project/lotus
	cd lotus
	git checkout master # or whichever version
	make debug
	sudo make install # or "cp lotus lotus-*" into your $PATH

### Running devnet on a remote host

The instructions below set up devnet on an Ubuntu machine. For example, we
followed them for `ubuntu@ec2-3-237-19-14.compute-1.amazonaws.com`.

As root:

	# disable and stop the existing lotus services
	systemctl disable lotus-daemon lotus-miner
	systemctl stop lotus-daemon lotus-miner

	# this is owned by another user possibly; delete it
	sudo rm -rf /var/tmp/filecoin-*

	# build-essential plus the list from filecoin docs
	apt install build-essential mesa-opencl-icd ocl-icd-opencl-dev gcc git bzr jq pkg-config curl clang build-essential hwloc libhwloc-dev wget
	snap install --classic go # apt has older versions

Then, as the regular user, build and install lotus as per the instructions
above.

Finally:

	git clone https://github.com/filecoin-project/dealbot
	cd dealbot
	go install ./cmd/devnet
	cd ..

	export LOTUS_PATH=/tmp/devnet-lotus
	export LOTUS_MINER_PATH=/tmp/devnet-miner
	mkdir -p $LOTUS_PATH $LOTUS_MINER_PATH

Finally, to run devnet, we run the command below inside the default `tmux` session:

	devnet

You can attach to it via `tmux a`, or create it via just `tmux`.

Note that, when restarting devnet, you probably have to empty the directories to
avoid errors:

	rm -rf $LOTUS_PATH/* $LOTUS_MINER_PATH/* localnet.json

When running, you can simply:

	source devnet/tunnel.sh

and then run the dealbot, e.g. via the integration tests.
