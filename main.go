package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/jonas-p/go-shp"
	"github.com/paulmach/orb"
	"github.com/paulmach/orb/geojson"
)

var (
	input    string
	output   string
	encoding string
	ndjson   bool
	pretty   bool
)

func init() {
	flag.StringVar(&input, "i", "", "input shapefile")
	flag.StringVar(&output, "o", "-", "output geojson file")
	flag.StringVar(&encoding, "e", "", "encoding of dbf")
	flag.BoolVar(&ndjson, "ndjson", false, "output as ndjson")
	flag.BoolVar(&pretty, "pretty", false, "pretty, no effect when output as ndjson")
	flag.Parse()
}

func main() {
	if input == "" {
		flag.Usage()
		os.Exit(1)
	}
	var options []shp.OptionFunc
	if encoding != "" {
		options = append(options, shp.WithDbfEncoding(encoding))
	}
	reader, err := shp.Open(input, options...)
	if err != nil {
		log.Fatalln(err)
	}
	fields := reader.Fields()
	collection := geojson.NewFeatureCollection()
	for reader.Next() {
		n, shape := reader.Shape()
		var attrs []shp.Attribute
		for k := range fields {
			attr := reader.ReadAttribute(n, k)
			if attr != nil {
				attrs = append(attrs, attr)
			}
		}
		collection.Features = append(collection.Features, ShapeToFeature(shape, attrs))
	}
	out, err := getOutput(output)
	if err != nil {
		log.Fatalln(err)
	}
	defer out.Close()
	encoder := json.NewEncoder(out)
	if !ndjson {
		if pretty {
			encoder.SetIndent("", "  ")
		}
		if err := encoder.Encode(collection); err != nil {
			log.Fatalln(err)
		}
	} else {
		for _, feature := range collection.Features {
			encoder.Encode(feature)
		}
	}
}

func getOutput(output string) (out io.WriteCloser, err error) {
	if output == "" || output == "-" {
		out = os.Stdout
		return
	}
	out, err = os.OpenFile(output, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	return
}

func ShapeToFeature(shape shp.Shape, attrs []shp.Attribute) *geojson.Feature {
	var g orb.Geometry
	switch s := shape.(type) {
	case *shp.Point:
		g = convertPoint(s)
	case *shp.PolyLine:
		if s.NumParts == 1 {
			g = convertLineString(s)
		} else if s.NumParts > 1 {
			g = convertMultiLineString(s)
		}
	case *shp.PolyLineZ:
		if s.NumParts == 1 {
			g = convertLineStringZ(s)
		} else if s.NumParts > 1 {
			g = convertMultiLineStringZ(s)
		}
	case *shp.Polygon:
		g = convertMultiPolygon(s)
	case *shp.MultiPoint:
		g = convertMultiPoint(s)
	default:
		panic(fmt.Sprintf("unsupported geometry type %v", s))
	}
	f := geojson.NewFeature(g)
	for _, attr := range attrs {
		f.Properties[attr.Name()] = attr.Value()
	}
	return f
}

func convertPoint(s *shp.Point) orb.Point {
	return orb.Point{s.X, s.Y}
}

func convertLineString(s *shp.PolyLine) orb.LineString {
	g := orb.LineString{}
	for _, p := range s.Points {
		g = append(g, convertPoint(&p))
	}
	return g
}

func convertLineStringZ(s *shp.PolyLineZ) orb.LineString {
	g := orb.LineString{}
	for _, p := range s.Points {
		g = append(g, convertPoint(&p))
	}
	return g
}

func convertMultiPoint(s *shp.MultiPoint) orb.MultiPoint {
	g := orb.MultiPoint{}
	for _, p := range s.Points {
		g = append(g, convertPoint(&p))
	}
	return g
}

func convertMultiLineString(s *shp.PolyLine) orb.MultiLineString {
	g := orb.MultiLineString{}
	for i, start := range s.Parts {
		var end int32
		if int32(i) < s.NumParts-1 {
			end = s.Parts[i+1]
		} else {
			end = s.NumPoints
		}
		l := orb.LineString{}
		for _, p := range s.Points[start:end] {
			l = append(l, convertPoint(&p))
		}
		g = append(g, l)
	}
	return g
}

func convertMultiLineStringZ(s *shp.PolyLineZ) orb.MultiLineString {
	g := orb.MultiLineString{}
	for i, start := range s.Parts {
		var end int32
		if int32(i) < s.NumParts-1 {
			end = s.Parts[i+1]
		} else {
			end = s.NumPoints
		}
		l := orb.LineString{}
		for _, p := range s.Points[start:end] {
			l = append(l, convertPoint(&p))
		}
		g = append(g, l)
	}
	return g
}

func convertMultiPolygon(s *shp.Polygon) orb.MultiPolygon {
	g := orb.MultiPolygon{}
	var poly orb.Polygon
	for i, start := range s.Parts {
		var end int32
		if int32(i) < s.NumParts-1 {
			end = s.Parts[i+1]
		} else {
			end = s.NumPoints
		}
		r := orb.Ring{}
		for _, p := range s.Points[start:end] {
			r = append(r, convertPoint(&p))
		}
		if i == 0 {
			poly = append(poly, r)
		} else if r.Orientation() == orb.CW {
			g = append(g, poly)
			poly = orb.Polygon{}
			poly = append(poly, r)
		} else {
			poly = append(poly, r)
		}
	}
	return g
}
