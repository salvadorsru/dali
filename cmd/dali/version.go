package main

import (
	"fmt"
	"runtime"
)

// version is set at build time with -ldflags "-X main.version=...".
var version = "dev"

func cmdVersion(args []string) error {
	fmt.Printf("dali %s\n", version)
	fmt.Printf("%s %s/%s\n", runtime.Version(), runtime.GOOS, runtime.GOARCH)
	return nil
}
