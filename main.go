package main

import (
	"context"
	"encoding/json"
	"fmt"
	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"time"
)

var (
	version = "0.0.6"
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
	httpClient := &http.Client{}
	req, _ := http.NewRequest("GET", "https://www.vattenfall.se/api/price/spot/pricearea/"+time.Now().Format("2006-01-02")+"/"+time.Now().AddDate(0, 0, 1).Format("2006-01-02")+"/SN3", nil)
	req.Header.Set("User-Agent", fmt.Sprintf("vattenfall-to-influxdb/%s (+https://github.com/rvoitenko/vattenfall-to-influxdb)", version))
	resp, err := httpClient.Do(req)
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("No response from request")
	}
	var result Response
	if err := json.Unmarshal(body, &result); err != nil {
		fmt.Println("Can not unmarshal JSON")
	}

	client := influxdb2.NewClient(os.Getenv("INFLUXDB_URL"), os.Getenv("INFLUXDB_TOKEN"))
	writeAPI := client.WriteAPIBlocking(os.Getenv("INFLUXDB_ORG"), os.Getenv("INFLUXDB_BUCKET"))
	for _, rec := range result {
		date, error := time.Parse("2006-01-02T15:04:05", rec.TimeStamp)
		if error != nil {
			fmt.Println(error)
			return
		}

		// Load the desired time zone, e.g., "Europe/Stockholm" or any other relevant time zone
		location, locErr := time.LoadLocation("Europe/Stockholm")
		if locErr != nil {
			fmt.Println(locErr)
			return
		}

		// Convert the parsed time to the loaded time zone
		dateInLocation := date.In(location)

		p := influxdb2.NewPoint("current",
			map[string]string{"unit": "price"},
			map[string]interface{}{"last": rec.Value},
			dateInLocation) // Use the adjusted time here
		writeAPI.WritePoint(context.Background(), p)
	}
	client.Close()
	fmt.Printf("%v: Tick\n", t)
}

func main() {
	intervalStr := os.Getenv("INTERVAL")
	interval := 30 // Default interval in minutes
	if intervalStr != "" {
		intervalInt, err := strconv.Atoi(intervalStr)
		if err == nil {
			interval = intervalInt
		} else {
			fmt.Println("Invalid INTERVAL value, using default 30 minutes")
		}
	}

	doEvery(time.Duration(interval)*time.Minute, pushToInflux)
}
