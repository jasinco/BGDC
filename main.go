package main

import (
	"flag"
	"log"
	"os"

	"github.com/jasinco/BGDC/core"
)

var (
	url         string
	path        string
	connections int
)

func init() {
	flag.IntVar(&connections, "con", 6, "Connections")
	flag.StringVar(&path, "o", "", "destination")
}

func main() {
	flag.Parse()
	url = flag.Arg(0)

	if url == "" {
		log.Print("URL is nil")
		os.Exit(1)
	}

	core.DownloadHandle(url, path, connections)
}
