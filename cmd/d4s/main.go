package main

import (
	"fmt"
	"os"

	"github.com/jr-k/d4s/internal/ui"
)

func main() {
	app := ui.NewApp()
	if err := app.Run(); err != nil {
		fmt.Printf("Error running D4s: %v\n", err)
		os.Exit(1)
	}
}
