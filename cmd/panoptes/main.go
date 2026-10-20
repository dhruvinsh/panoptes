// Command panoptes streams PC stats to an ESP32 "Cheap Yellow Display" over USB serial.
package main

import (
	"os"

	"github.com/dhruvinsh/panoptes/internal/cli"
)

// version is set at build time: go build -ldflags "-X main.version=v1.2.3".
var version = "dev"

func main() {
	os.Exit(cli.Main(version, os.Args[1:], os.Stdout, os.Stderr))
}
