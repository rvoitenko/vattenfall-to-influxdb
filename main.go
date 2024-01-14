package main

import (
    "context"
    "encoding/json"
    "fmt"
    "github.com/go-resty/resty/v2"
    influxdb2 "github.com/influxdata/influxdb-client-go/v2"
    "os"
    "strconv"
    "time"

    _ "time/tzdata" // Embed the IANA Time Zone database
)

var (
    version = "0.0.19"
    debug   bool
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
    client := resty.New()

    // Read the PRICE_AREA environment variable, default to "SN3" if not set
    priceArea := os.Getenv("PRICE_AREA")
    if priceArea == "" {
        fmt.Println("Error: PRICE_AREA environment variable is not set")
        os.Exit(1)
    }

    endOfDay := time.Now().Truncate(24 * time.Hour).Add(24*time.Hour - 1*time.Second)
    url := fmt.Sprintf("https://www.vattenfall.se/api/price/spot/pricearea/%s/%s/%s?_=%d",
        time.Now().Format("2006-01-02"),
        endOfDay.AddDate(0, 0, 1).Format("2006-01-02"),
        priceArea,
        time.Now().UnixNano()) // UnixNano returns a unique number

    resp, err := client.R().
        SetHeader("User-Agent", fmt.Sprintf("vattenfall-to-influxdb/%s (+https://github.com/rvoitenko/vattenfall-to-influxdb)", version)).
        SetHeader("Cache-Control", "no-cache, no-store, must-revalidate").
        SetHeader("Pragma", "no-cache").
        Get(url)

    if err != nil {
        fmt.Println("Error on request:", err)
        return
    }

    var result Response
    if err := json.Unmarshal(resp.Body(), &result); err != nil {
        fmt.Println("Can not unmarshal JSON", err)
        return
    }

    if debug {
        fmt.Printf("URL: %s\n", url)
        fmt.Println(result)
    } else {
        influxClient := influxdb2.NewClient(os.Getenv("INFLUXDB_URL"), os.Getenv("INFLUXDB_TOKEN"))
        writeAPI := influxClient.WriteAPIBlocking(os.Getenv("INFLUXDB_ORG"), os.Getenv("INFLUXDB_BUCKET"))
        for _, rec := range result {
            date, error := time.Parse("2006-01-02T15:04:05", rec.TimeStamp)
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
        influxClient.Close()
    }
    fmt.Printf("%v: Tick\n", t)
}

func main() {
    location, err := time.LoadLocation("Europe/Stockholm")
    if err != nil {
        fmt.Println("Error loading location 'Europe/Stockholm':", err)
        os.Exit(1)
    }
    time.Local = location

    intervalStr := os.Getenv("INTERVAL")
    interval := 1800
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
