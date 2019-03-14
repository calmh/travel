package main

import (
	"encoding/csv"
	"encoding/json"
	"flag"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
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
	purpose  string
	lat, lng float64
}

var colors = []string{
	"#a6cee3",
	"#1f78b4",
	"#b2df8a",
	"#33a02c",
	"#fb9a99",
	"#e31a1c",
	"#fdbf6f",
	"#ff7f00",
	"#cab2d6",
	"#6a3d9a",
	"#ffff99",
	"#b15928",
}

var yearToColor = map[int]string{}
var nextColor = 0

func colorForYear(t time.Time) string {
	if c, ok := yearToColor[t.Year()]; ok {
		return c
	}
	c := colors[nextColor]
	nextColor = (nextColor + 1) % len(colors)
	yearToColor[t.Year()] = c
	return c
}

func visitFromStrings(fs []string) *visit {
	if len(fs) != 5 {
		return nil
	}

	var v visit

	v.when, _ = time.Parse("2006-01-02", strings.TrimSpace(fs[0]))
	v.purpose = strings.TrimSpace(fs[1])
	v.address = strings.TrimSpace(fs[2])
	v.lat, _ = strconv.ParseFloat(strings.TrimSpace(fs[3]), 64)
	v.lng, _ = strconv.ParseFloat(strings.TrimSpace(fs[4]), 64)

	if v.lat == 0 && v.lng == 0 {
		v.lat, v.lng, v.address = lookup(v.address)
	}

	return &v
}

func (v *visit) strings() []string {
	return []string{
		v.when.Format("2006-01-02"),
		v.purpose,
		v.address,
		strconv.FormatFloat(v.lat, 'f', 4, 64),
		strconv.FormatFloat(v.lng, 'f', 4, 64),
	}
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
			"marker-symbol": v.purpose,
			"marker-color":  colorForYear(v.when),
			"date":          v.when.Format("2006-01-02"),
			"name":          v.address,
		},
	})
}

type visitList []*visit

func (l visitList) Less(a, b int) bool {
	if !l[a].when.Equal(l[b].when) {
		return l[a].when.Before(l[b].when)
	}
	return l[a].address < l[b].address
}
func (l visitList) Swap(a, b int) {
	l[a], l[b] = l[b], l[a]
}
func (l visitList) Len() int {
	return len(l)
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
	in, err := r.Read()
	for err == nil {
		visits = append(visits, visitFromStrings(in))
		in, err = r.Read()
	}
	fd.Close()

	sort.Sort(visitList(visits))

	purposes := make(map[string]struct{})
	for _, v := range visits {
		purposes[v.purpose] = struct{}{}
	}

	fd, err = os.Create(*file)
	if err != nil {
		log.Fatal(err)
	}
	w := csv.NewWriter(fd)
	for _, v := range visits {
		w.Write(v.strings())
	}
	w.Flush()
	fd.Close()

	fname := strings.Replace(*file, ".csv", ".geojson", 1)
	saveVisits(visits, fname)

	for purpose := range purposes {
		dir := filepath.Dir(fname)
		base := filepath.Base(fname)
		tname := filepath.Join(dir, purpose+"-"+base)
		saveVisits(filterByPurpose(visits, purpose), tname)
	}
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

func lookup(search string) (lat, lng float64, addr string) {
	key := os.Getenv("MAPS_API_KEY")
	query := url.Values{
		"key":     []string{key},
		"address": []string{search},
	}
	resp, err := http.Get("https://maps.googleapis.com/maps/api/geocode/json?" + query.Encode())
	if err != nil {
		log.Printf("Lookup: resolving %s: %v:", search, err)
		return 0, 0, search
	}
	defer resp.Body.Close()
	if resp.StatusCode > 299 {
		log.Printf("Lookup: resolving %s: %s", search, resp.Status)
		return 0, 0, search
	}

	bs, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Lookup: resolving %s: %v:", search, err)
		return 0, 0, search
	}

	var res result
	err = json.Unmarshal(bs, &res)
	if err != nil {
		log.Printf("Lookup: resolving %s: %v:", search, err)
		log.Printf("Data: %s", bs)
		return 0, 0, search
	}

	if len(res.Results) == 0 {
		log.Printf("Lookup: resolving %s: no results", search)
		log.Printf("Data: %s", bs)
		return 0, 0, search
	}

	loc := res.Results[0].Geometry.Location
	add := res.Results[0].Address
	return loc.Lat, loc.Lng, add
}

func filterByPurpose(vs []*visit, purpose string) []*visit {
	var res []*visit
	for _, v := range vs {
		if v.purpose == purpose {
			res = append(res, v)
		}
	}
	return res
}
