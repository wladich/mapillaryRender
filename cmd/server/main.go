package main

import (
	"flag"
	"fmt"
	"github.com/paulmach/orb/maptile"
	"github.com/wladich/mapillaryRender/pkg/render"
	"log"
	"net/http"
	"strconv"
	"strings"
)

var apiBaseURL, apiBaseURLZ14, accessToken string

const tileSize = 1024

func handleRequest(resp http.ResponseWriter, req *http.Request) {
	if req.Method != "GET" {
		http.Error(resp, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	path := strings.Trim(req.URL.Path, "/")
	pathElements := strings.Split(path, "/")
	if len(pathElements) != 3 {
		http.NotFound(resp, req)
		return
	}
	tileZ, errZ := strconv.ParseUint(pathElements[0], 10, 32)
	tileX, errX := strconv.ParseUint(pathElements[1], 10, 32)
	tileY, errY := strconv.ParseUint(pathElements[2], 10, 32)

	if errX != nil || errY != nil || errZ != nil {
		http.NotFound(resp, req)
		return
	}
	tile := maptile.Tile{X: uint32(tileX), Y: uint32(tileY), Z: maptile.Zoom(tileZ)}
	if !tile.Valid() {
		http.NotFound(resp, req)
		return
	}
	imageData, err := render.Tile(tile, tileSize, apiBaseURL, apiBaseURLZ14, accessToken)
	if err != nil {
		http.Error(resp, "Server error", http.StatusInternalServerError)
		log.Printf("Error rendering tile: %v", err)
		return
	}
	resp.Header().Add("content-type", "image/png")
	resp.Header().Add("content-length", strconv.Itoa(len(imageData)))
	resp.Write(imageData)
}

func limitNumClients(f http.HandlerFunc, maxClients int) http.HandlerFunc {
	// Counting semaphore using a buffered channel
	sema := make(chan struct{}, maxClients)

	return func(w http.ResponseWriter, req *http.Request) {
		if len(sema) == maxClients {
			log.Printf("Number of concurrent clients reached maxClient (%v), the request will be blocked", maxClients)
		}
		sema <- struct{}{}
		defer func() { <-sema }()
		f(w, req)
	}
}

func main() {
	port := flag.Int("port", 8080, "port to listen")
	host := flag.String("host", "127.0.0.1", "address to bind to")
	maxClients := flag.Int("clients", 1, "maximum number of concurrently served requests")
	maxNetworkJobs := flag.Int("net-jobs", 1, "maximum number simultaneous data requests")
	maxCPUJobs := flag.Int("cpu-jobs", 1, "maximum number of simultaneous rendering jobs")
	maxNetworkRequestRetries := flag.Int("net-retries", 3, "maximum number of HTTP requests retries")
	flag.StringVar(&apiBaseURL, "api", "https://tiles.mapillary.com/maps/vtp/mly1_public/2", "Base API URL")
	flag.StringVar(&apiBaseURLZ14, "api-z14", "", "Base API URL for detailed level (z=14).Default is same as -api")
	flag.StringVar(&accessToken, "token", "", "Mapillary access token")
	flag.Parse()

	if len(apiBaseURLZ14) == 0 {
		apiBaseURLZ14 = apiBaseURL
	}
	log.Printf("Starting server with parameter: host=%v, port=%v, maxClients=%v",
		*host, *port, *maxClients)
	render.SetMaxCPUJobs(*maxCPUJobs)
	render.SetMaxNetworkJobs(*maxNetworkJobs)
	render.SetMaxNetworkRequestRetries(*maxNetworkRequestRetries)
	http.HandleFunc("/", limitNumClients(handleRequest, *maxClients))
	log.Printf("Serving at %s:%d", *host, *port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf("%s:%d", *host, *port), nil))
}
