package main

import (
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"
)

type result struct {
	Results []struct {
		Address  string `json:"formatted_address"`
		Geometry struct {
			Location struct {
				Lat float64
				Lng float64
			}
		}
	}
}

type visit struct {
	address  string
	when     time.Time
	lat, lng float64
}

func visitFromStrings(fs []string) *visit {
	if len(fs) != 5 {
		return nil
	}

	var v visit

	v.when, _ = time.Parse("2006-01-02", strings.TrimSpace(fs[0]))
	v.address = strings.TrimSpace(fs[2])
	v.lat, _ = strconv.ParseFloat(strings.TrimSpace(fs[3]), 64)
	v.lng, _ = strconv.ParseFloat(strings.TrimSpace(fs[4]), 64)

	return &v
}

func (v *visit) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"type": "Feature",
		"geometry": map[string]interface{}{
			"type": "Point",
			"coordinates": []float64{
				float64(int(10000*v.lng)) / 10000,
				float64(int(10000*v.lat)) / 10000,
			},
		},
		"properties": map[string]interface{}{
			"date": v.when.Format("2006-01-02"),
			"name": v.address,
		},
	})
}

func main() {
	file := flag.String("file", "travel.csv", "CSV file name")
	flag.Parse()

	fd, err := os.Open(*file)
	if err != nil {
		log.Fatal(err)
	}

	r := csv.NewReader(fd)
	var visits []*visit
	seenCoords := make(map[string]struct{})
	for {
		in, err := r.Read()
		if err != nil {
			break
		}
		visit := visitFromStrings(in)
		coords := fmt.Sprintf("%.04f,%.04f", visit.lat, visit.lat)
		if _, ok := seenCoords[coords]; ok {
			continue
		}
		seenCoords[coords] = struct{}{}
		visits = append(visits, visit)
	}
	fd.Close()

	slices.SortFunc(visits, func(a, b *visit) int {
		return a.when.Compare(b.when)
	})

	fname := strings.Replace(*file, ".csv", ".geojson", 1)
	saveVisits(visits, fname)
}

func saveVisits(visits []*visit, fname string) {
	geojson := map[string]interface{}{
		"type":     "FeatureCollection",
		"features": visits,
	}
	bs, _ := json.MarshalIndent(geojson, "", "  ")

	fd, err := os.Create(fname)
	if err != nil {
		log.Fatal(err)
	}
	fd.Write(bs)
	fd.Close()
}
