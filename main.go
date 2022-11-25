package main

import (
	"flag"

	"github.com/jasinco/BGDC/core"
)

var (
	url  string
	path string
)

func init() {
	flag.StringVar(&path, "o", "", "destination")
}

func main() {
	flag.Parse()
	url = flag.Arg(0)
	core.DownloadHandle(url, path)
}
