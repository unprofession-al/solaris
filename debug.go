package main

import (
	"fmt"
	"os"
)

func debug(out string) {
	if rootDebug {
		fmt.Fprintf(os.Stderr, out)
	}
}
