package devnet

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

func runLotusNode(ctx context.Context) {
	var stdout, stderr bytes.Buffer
	lotusNodeCmd := "rm -rf ~/.devlotus && export LOTUS_PATH=~/.devlotus && rm -rf localnet.json &&  lotus-seed genesis new localnet.json  && lotus-seed pre-seal --sector-size 2048 --num-sectors 10 && lotus-seed genesis add-miner localnet.json ~/.genesis-sectors/pre-seal-t01000.json && lotus daemon --lotus-make-genesis=dev.gen --genesis-template=localnet.json --bootstrap=false"

	cmd := exec.CommandContext(ctx, "sh", "-c", lotusNodeCmd)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Start()
	if err != nil {
		log.Fatal(err)
	}

	err = cmd.Wait()
	if err != nil {
		log.Println("waiting on lotus node:", err)
	}
}

func runMiner(ctx context.Context) {
	time.Sleep(5 * time.Second) // wait for lotus node to run

	var stdout, stderr bytes.Buffer
	lotusMinerCmd := "rm -rf ~/.lotusminer && export LOTUS_PATH=~/.devLotus && lotus wallet import ~/.genesis-sectors/pre-seal-t01000.key && lotus-miner init --genesis-miner --actor=t01000 --sector-size=2048 --pre-sealed-sectors=~/.genesis-sectors --pre-sealed-metadata=~/.genesis-sectors/pre-seal-t01000.json --nosync && lotus-miner run --nosync"

	cmd := exec.CommandContext(ctx, "sh", "-c", lotusMinerCmd)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Start()
	if err != nil {
		log.Fatal(err)
	}

	err = cmd.Wait()
	if err != nil {
		log.Println("waiting on lotus miner:", err)
	}
}

func publishDealsPeriodicallyCmd(ctx context.Context) {
	var stdout, stderr bytes.Buffer
	publishDealsCmd := "lotus-miner storage-deals pending-publish --publish-now"

	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(30 * time.Second):
		default:
		}

		cmd := exec.CommandContext(ctx, "sh", "-c", publishDealsCmd)
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		err := cmd.Start()
		if err != nil {
			continue
		}

		cmd.Wait()
	}
}

func setDefaultWalletCmd(ctx context.Context) {
	var stdout, stderr bytes.Buffer
	setDefaultWalletCmd := "lotus wallet list | grep t3 | awk '{print $1}' | xargs lotus wallet set-default"

	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(30 * time.Second):
		default:
		}

		cmd := exec.CommandContext(ctx, "sh", "-c", setDefaultWalletCmd)
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		err := cmd.Start()
		if err != nil {
			fmt.Println("error", err.Error())
			continue
		}

		err = cmd.Wait()
		if err != nil {
			continue
		}
	}
}

func Main() {
	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	wg.Add(4)
	go func() {
		runLotusNode(ctx)
		wg.Done()
	}()

	go func() {
		runMiner(ctx)
		wg.Done()
	}()

	go func() {
		publishDealsPeriodicallyCmd(ctx)
		wg.Done()
	}()

	go func() {
		setDefaultWalletCmd(ctx)
		wg.Done()
	}()

	// setup a signal handler to cancel the context
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, syscall.SIGTERM, syscall.SIGINT)
	select {
	case <-interrupt:
		log.Println("closing as we got interrupt")
		cancel()
	case <-ctx.Done():
	}

	wg.Wait()
}
