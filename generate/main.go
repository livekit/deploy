package main

import (
	"fmt"
	"math/rand"
	"os"
	"path"
	"time"

	"github.com/urfave/cli/v2"
)

const (
	filePerms = 0644
)

func init() {
	rand.Seed(time.Now().Unix())
}

func main() {
	app := &cli.App{
		Name:    "generate",
		Usage:   "Generates Configurations for LiveKit",
		Version: "1.0.0",
		Action:  startGenerator,
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:  "local",
				Usage: "generates local config",
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		fmt.Println(err)
	}
}

func startGenerator(c *cli.Context) error {
	if c.Bool("local") {
		return generateLocal()
	}
	return generateProduction()
}

// map differences between docker environment
func outputPath(file string) string {
	if _, err := os.Stat("/.dockerenv"); err != nil {
		return file
	}
	return path.Join("/output", file)
}
