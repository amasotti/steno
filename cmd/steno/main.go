package main

import (
	"context"
	"fmt"
	"os"

	"github.com/amasotti/steno/internal/cli"
)

func main() {
	if err := cli.Run(context.Background(), os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintf(os.Stderr, "steno: %v\n", err)
		os.Exit(1)
	}
}
