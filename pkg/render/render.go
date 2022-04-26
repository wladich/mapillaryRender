package render

import (
	"errors"
	"fmt"
	"github.com/paulmach/orb"
	"github.com/paulmach/orb/encoding/mvt"
	"github.com/paulmach/orb/maptile"
	"github.com/ungerik/go-cairo"
	"io/ioutil"
	"net/http"
	"time"
)

var colorNotPano = [3]float64{40. / 255, 150. / 255, 30. / 255}
var colorPano = [3]float64{0, 100. / 255, 30. / 255}

func drawPoints(
	layer *mvt.Layer,
	surface *cairo.Surface,
	radius float64,
	dataToImageScale float64,
	rasterOffsetX float64,
	rasterOffsetY float64,
	drawPano bool,
	drawNotPano bool,
) {
	surface.SetLineWidth(radius * 2)
	surface.SetLineCap(cairo.LINE_CAP_ROUND)

	var n int32 = 0

	for _, feature := range layer.Features {
		isPano := feature.Properties.MustBool("is_pano")
		if (isPano && !drawPano) || (!isPano && !drawNotPano) {
			continue
		}
		geom := feature.Geometry
		point := geom.(orb.Point)
		x := point.X()*dataToImageScale + rasterOffsetX
		y := point.Y()*dataToImageScale + rasterOffsetY
		surface.MoveTo(x, y)
		surface.LineTo(x, y)
		n++
		if n > 100 {
			surface.Stroke()
			n = 0
		}
	}
	surface.Stroke()
}

func drawLines(
	layer *mvt.Layer,
	surface *cairo.Surface,
	lineWidth float64,
	dataToImageScale float64,
	rasterOffsetX float64,
	rasterOffsetY float64,
	drawPano bool,
	drawNotPano bool,
) {
	surface.SetLineWidth(lineWidth)
	surface.SetLineCap(cairo.LINE_CAP_ROUND)
	surface.SetLineJoin(cairo.LINE_JOIN_ROUND)
	var n int32

	for _, feature := range layer.Features {
		isPano := feature.Properties.MustBool("is_pano")
		if (isPano && !drawPano) || (!isPano && !drawNotPano) {
			continue
		}
		var lines []orb.LineString
		geom := feature.Geometry
		if geom.GeoJSONType() == "LineString" {
			lines = append(lines, geom.(orb.LineString))
		} else {
			lines = geom.(orb.MultiLineString)
		}

		for _, line := range lines {
			lineLen := len(line)
			if lineLen == 0 {
				continue
			}
			pt := line[0]
			surface.MoveTo(pt.X()*dataToImageScale+rasterOffsetX, pt.Y()*dataToImageScale+rasterOffsetY)
			if lineLen > 1 {
				line = line[1:]
			}
			for _, pt := range line {
				surface.LineTo(pt.X()*dataToImageScale+rasterOffsetX, pt.Y()*dataToImageScale+rasterOffsetY)
				n++
			}
			if n > 100 {
				surface.Stroke()
				n = 0
			}
		}
	}
	surface.Stroke()
}

func renderFromMvt(mvtData *[]byte, tileSize uint32, dataScale uint32, offsetX float64, offsetY float64, detailed bool) ([]byte, error) {
	layers, err := mvt.Unmarshal(*mvtData)
	if err != nil {
		return nil, err
	}
	surface := cairo.NewSurface(cairo.FORMAT_ARGB32, int(tileSize), int(tileSize))
	defer surface.Finish()
	surface.SetAntialias(4) // CAIRO_ANTIALIAS_FAST = 4
	surface.SetSourceRGB(colorNotPano[0], colorNotPano[1], colorNotPano[2])
	for _, layer := range layers {
		dataToImageScale := float64(tileSize*dataScale) / float64(layer.Extent)
		switch layer.Name {
		case "overview":
			drawPoints(layer, surface, 6, dataToImageScale, offsetX, offsetY, true, true)
		case "sequence":
			var lineWidth float64
			if detailed {
				lineWidth = 2
			} else {
				lineWidth = 6
			}
			drawLines(layer, surface, lineWidth, dataToImageScale, offsetX, offsetY, false, true)
		case "image":
			drawPoints(layer, surface, 6, dataToImageScale, offsetX, offsetY, false, true)
		default:
			continue
		}
	}
	surface.SetSourceRGB(colorPano[0], colorPano[1], colorPano[2])
	for _, layer := range layers {
		dataToImageScale := float64(tileSize*dataScale) / float64(layer.Extent)
		switch layer.Name {
		case "sequence":
			var lineWidth float64
			if detailed {
				lineWidth = 2
			} else {
				lineWidth = 6
			}
			drawLines(layer, surface, lineWidth, dataToImageScale, offsetX, offsetY, true, false)
		case "image":
			drawPoints(layer, surface, 6, dataToImageScale, offsetX, offsetY, true, false)
		default:
			continue
		}
	}
	res, status := surface.WriteToPNGStream()
	if status != 0 {
		return nil, fmt.Errorf("error %d saving PNG", status)
	}

	return res, nil
}

// Tile fetches mvt tile from server and renders an image
func Tile(tileInd maptile.Tile, tileSize uint32, apiURL string, apiAccessToken string) ([]byte, error) {
	if !tileInd.Valid() {
		return nil, errors.New("invalid tileInd index")
	}
	if tileInd.Z > 22 {
		return nil, errors.New("tileInd Z is too big (max 22)")
	}
	dataX := tileInd.X
	dataY := tileInd.Y
	dataZ := int(tileInd.Z)
	var dataScale uint32 = 1
	var offsetX float64 = 0
	var offsetY float64 = 0
	if dataZ > 14 {
		offsetZ := tileInd.Z - 14
		dataScale = 1 << offsetZ
		dataZ = 14
		dataX /= dataScale
		dataY /= dataScale
		offsetX = float64(-tileSize * (tileInd.X - dataX*dataScale))
		offsetY = float64(-tileSize * (tileInd.Y - dataY*dataScale))
	}
	url := fmt.Sprintf("%s/%d/%d/%d", apiURL, dataZ, dataX, dataY)
	if apiAccessToken != "" {
		url += fmt.Sprintf("?access_token=%s", apiAccessToken)
	}
	client := http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("request for MVT tileInd returned HTTP error %d", resp.StatusCode)
	}
	mvtData, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return renderFromMvt(&mvtData, tileSize, dataScale, offsetX, offsetY, dataZ == 14)
}
