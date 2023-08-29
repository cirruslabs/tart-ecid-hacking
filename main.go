package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path"
)

func main() {
	vmName := "ventura-base"
	outputFile := "data.cvs"

	args := os.Args[1:]
	if len(args) > 1 {
		outputFile = args[1]
	}
	if len(args) > 0 {
		vmName = args[0]
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Fatal(err)
	}

	tartBinaryPath := path.Join(homeDir, "workspace", "tart", ".build", "debug", "tart")

	f, err := os.Create(outputFile)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	for i := 0; i < 50_000; i++ {
		info, err := collectForVM(context.Background(), tartBinaryPath, vmName)
		if err != nil {
			panic(err)
		}
		line := fmt.Sprintf("%s\t%s", info.ECID, info.HardwareModelBase64)
		println(line)
		_, _ = f.WriteString(line + "\n")
	}
}
