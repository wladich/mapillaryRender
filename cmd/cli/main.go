package main

import (
	"flag"
	"fmt"
	"github.com/paulmach/orb/maptile"
	"github.com/wladich/mapillaryRender/pkg/render"
	"io/ioutil"
	"os"
	"time"
)

func exitWithWrongArgs(msg string) {
	_, err := fmt.Fprintf(os.Stderr, "%s\n\n", msg)
	if err != nil {
		panic(err)
	}
	flag.Usage()
	os.Exit(1)
}

func main() {
	tileX := flag.Uint("x", 0, "Tile X")
	tileY := flag.Uint("y", 0, "Tile Y")
	tileZ := flag.Uint("z", 0, "Tile Z")
	tileSize := flag.Uint("tileSize", 1024, "Image size")
	apiBaseURL := flag.String("api", "https://tiles.mapillary.com/maps/vtp/mly1_public/2", "Base API URL")
	accessToken := flag.String("token", "", "Mapillary access token")
	outFile := flag.String("out", "", "Output file name")
	flag.Parse()
	if *outFile == "" {
		exitWithWrongArgs("Output file name not set")

	}
	start := time.Now()
	tile := maptile.Tile{X: uint32(*tileX), Y: uint32(*tileY), Z: maptile.Zoom(*tileZ)}
	if !tile.Valid() {
		exitWithWrongArgs("Invalid tile index")
	}
	imageData, err := render.Tile(tile, uint32(*tileSize), *apiBaseURL, *apiBaseURL, *accessToken)
	if err != nil {
		panic(err)
	}
	if err := ioutil.WriteFile(*outFile, imageData, 0644); err != nil {
		panic(err)
	}
	fmt.Printf("Time consumed: %s\n", time.Since(start))
}
