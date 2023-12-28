package main

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"time"

	_ "time/tzdata" // Add this line to embed the IANA Time Zone database
)

var (
	version = "0.0.12"
	debug   bool // Add this line for the debug flag
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
	endOfDay := time.Now().Truncate(24 * time.Hour).Add(24*time.Hour - 1*time.Second)
	url := "https://www.vattenfall.se/api/price/spot/pricearea/" + time.Now().Format("2006-01-02") + "/" + endOfDay.AddDate(0, 0, 1).Format("2006-01-02") + "/SN3"
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("User-Agent", fmt.Sprintf("vattenfall-to-influxdb/%s (+https://github.com/rvoitenko/vattenfall-to-influxdb)", version))
	resp, err := httpClient.Do(req)
	if err != nil {
		fmt.Println("Error on request:", err)
		return
	}
	defer resp.Body.Close()

	var reader io.Reader
	switch resp.Header.Get("Content-Encoding") {
	case "gzip":
		reader, err = gzip.NewReader(resp.Body)
		if err != nil {
			fmt.Println("Error on gzip decompression:", err)
			return
		}
	default:
		reader = resp.Body
	}

	body, err := ioutil.ReadAll(reader)
	if err != nil {
		fmt.Println("Error reading response body:", err)
		return
	}

	var result Response
	if err := json.Unmarshal(body, &result); err != nil {
		fmt.Println("Can not unmarshal JSON")
	}

	if debug {
		fmt.Printf("URL: %s\n", url)
		fmt.Println(result) // Print the result for debugging
	} else {
		client := influxdb2.NewClient(os.Getenv("INFLUXDB_URL"), os.Getenv("INFLUXDB_TOKEN"))
		writeAPI := client.WriteAPIBlocking(os.Getenv("INFLUXDB_ORG"), os.Getenv("INFLUXDB_BUCKET"))
		for _, rec := range result {
			location, locErr := time.LoadLocation("Europe/Stockholm")
			if locErr != nil {
				fmt.Println(locErr)
				return
			}

			date, error := time.ParseInLocation("2006-01-02T15:04:05", rec.TimeStamp, location)
			if error != nil {
				fmt.Println(error)
				return
			}

			p := influxdb2.NewPoint("current",
				map[string]string{"unit": "price"},
				map[string]interface{}{"last": rec.Value},
				date)
			writeAPI.WritePoint(context.Background(), p)
		}
		client.Close()
	}
	fmt.Printf("%v: Tick\n", t)
}

func main() {
	intervalStr := os.Getenv("INTERVAL")
	interval := 1800 // Default interval in seconds
	if intervalStr != "" {
		intervalInt, err := strconv.Atoi(intervalStr)
		if err == nil {
			interval = intervalInt
		} else {
			fmt.Println("Invalid INTERVAL value, using default 30 seconds")
		}
	}

	debugStr := os.Getenv("DEBUG")
	if debugStr != "" {
		debugBool, err := strconv.ParseBool(debugStr)
		if err == nil {
			debug = debugBool
		} else {
			fmt.Println("Invalid DEBUG value, should be either true or false, using default false")
		}
	}

	doEvery(time.Duration(interval)*time.Second, pushToInflux)
}
