package main

import (
	"context"
	"fmt"
	"log"
	"os"
)

func main() {
	tartBinaryPath := "/Users/fedor/workspace/tart/.build/debug/tart"

	f, err := os.Create("data.cvs")
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	for i := 0; i < 2_000; i++ {
		info, err := collectForVM(context.Background(), tartBinaryPath, "ventura-xcode")
		if err != nil {
			panic(err)
		}
		line := fmt.Sprintf("%s\t%s", info.ECID, info.HardwareModelBase64)
		println(line)
		_, _ = f.WriteString(line + "\n")
	}
}
