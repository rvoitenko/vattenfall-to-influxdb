package main

import (
	"context"
	"encoding/json"
	"fmt"
	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"io/ioutil"
	"net/http"
	"os"
	"time"
)

type Response []struct {
	TimeStamp     string  `json:"TimeStamp"`
	TimeStampDay  string  `json:"TimeStampDay"`
	TimeStampHour string  `json:"TimeStampHour"`
	Value         float64 `json:"Value"`
	PriceArea     string  `json:"PriceArea"`
	Unit          string  `json:"Unit"`
}

func doEvery(d time.Duration, f func(time.Time)) {
	for x := range time.Tick(d) {
		f(x)
	}
}

func pushToInflux(t time.Time) {
	resp, _ := http.Get("https://www.vattenfall.se/api/price/spot/pricearea/" + time.Now().Format("2006-01-02") + "/" + time.Now().AddDate(0, 0, 1).Format("2006-01-02") + "/SN3")
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body) // response body is []byte
	if err != nil {
		fmt.Println("No response from request")
	}
	var result Response
	if err := json.Unmarshal(body, &result); err != nil { // Parse []byte to the go struct pointer
		fmt.Println("Can not unmarshal JSON")
	}

	// https://github.com/influxdata/influxdb-client-go
	client := influxdb2.NewClient(os.Getenv("INFLUXDB_URL"), os.Getenv("INFLUXDB_TOKEN"))
	writeAPI := client.WriteAPIBlocking(os.Getenv("INFLUXDB_ORG"), os.Getenv("INFLUXDB_BUCKET"))
	for _, rec := range result {
		date, error := time.Parse("2006-01-02T15:04:05", rec.TimeStamp)
		if error != nil {
			fmt.Println(error)
			return
		}
		p := influxdb2.NewPoint("current",
			map[string]string{"unit": "price"},
			map[string]interface{}{"last": rec.Value},
			date.Add(-1*time.Hour))
		writeAPI.WritePoint(context.Background(), p)
	}
	client.Close()
	fmt.Printf("%v: Tick\n", t)
}

func main() {
	doEvery(30*time.Minute, pushToInflux)
}
