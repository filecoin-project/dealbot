package devnet

import (
	"context"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"
)

func runShell(ctx context.Context, name string, commands []string) {
	logFile, err := os.Create(name + ".log")
	if err != nil {
		log.Fatal(err)
	}
	defer logFile.Close()

	src := strings.Join(commands, " && ")
	cmd := exec.CommandContext(ctx, "sh", "-c", src)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	if err := cmd.Run(); err != nil {
		log.Printf("%s; check %s for details", err, logFile.Name())
	}
}

func runLotusNode(ctx context.Context) {
	lotusNodeCmds := []string{
		"lotus-seed genesis new localnet.json",
		"lotus-seed pre-seal --sector-size 2048 --num-sectors 10",
		"lotus-seed genesis add-miner localnet.json ~/.genesis-sectors/pre-seal-t01000.json",
		"lotus daemon --lotus-make-genesis=dev.gen --genesis-template=localnet.json --bootstrap=false",
	}

	runShell(ctx, "lotus-daemon", lotusNodeCmds)
}

func runMiner(ctx context.Context) {
	time.Sleep(5 * time.Second) // wait for lotus node to run

	lotusMinerCmds := []string{
		"lotus wallet import ~/.genesis-sectors/pre-seal-t01000.key",
		"lotus-miner init --genesis-miner --actor=t01000 --sector-size=2048 --pre-sealed-sectors=~/.genesis-sectors --pre-sealed-metadata=~/.genesis-sectors/pre-seal-t01000.json --nosync",
		"lotus-miner run --nosync",
	}

	runShell(ctx, "lotus-miner", lotusMinerCmds)
}

func publishDealsPeriodicallyCmd(ctx context.Context) {
	publishDealsCmd := "lotus-miner storage-deals pending-publish --publish-now"

	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(5 * time.Second):
		}

		cmd := exec.CommandContext(ctx, "sh", "-c", publishDealsCmd)
		_, err := cmd.CombinedOutput()
		if err != nil {
			continue
		}

	}
}

func setDefaultWalletCmd(ctx context.Context) {
	setDefaultWalletCmd := "lotus wallet list | grep t3 | awk '{print $1}' | xargs lotus wallet set-default"

	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(5 * time.Second):
		}

		cmd := exec.CommandContext(ctx, "sh", "-c", setDefaultWalletCmd)
		_, err := cmd.CombinedOutput()
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
