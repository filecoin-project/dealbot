package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"

	"github.com/filecoin-project/dealbot/controller/webutil"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Printf("Must specify source directory")
		os.Exit(1)
	}

	cmd := exec.Command("npm", "install")
	cmd.Dir = os.Args[1]
	err := cmd.Run()
	if err != nil {
		fmt.Printf("Failed to install frontend dependencies: %v\n", err)
		if _, err := os.Stat(path.Join(os.Args[1], "node_modules")); os.IsNotExist(err) {
			os.Exit(1)
		}
	}

	data := webutil.Compile(os.Args[1], true)
	if len(os.Args) < 3 {
		fmt.Printf("%s\n", data)
	}
	ioutil.WriteFile(os.Args[2], []byte(data), 0644)
}
