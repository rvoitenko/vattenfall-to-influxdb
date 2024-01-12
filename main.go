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
    version = "0.0.17"
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

    endOfDay := time.Now().Truncate(24 * time.Hour).Add(24*time.Hour - 1*time.Second)
    url := "https://www.vattenfall.se/api/price/spot/pricearea/" + time.Now().Format("2006-01-02") + "/" + endOfDay.AddDate(0, 0, 2).Format("2006-01-02") + "/SN3"

    resp, err := client.R().
        SetHeader("User-Agent", fmt.Sprintf("vattenfall-to-influxdb/%s (+https://github.com/rvoitenko/vattenfall-to-influxdb)", version)).
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
