package main

import (
	"fmt"
	"log/slog"
	"os"

	"simple/cmd/api"
)

func main() {
	fmt.Println("\x1b[1;34mSimple to Go ...\x1b[0m")
	if err := api.Run(); err != nil {
		slog.Error("server exited", "err", err)
		os.Exit(1)
	}
}
