package main

import (
    "context"
    "encoding/json"
    "fmt"
    "io/ioutil"
    "net/http"
    "os"
    "strconv"
    "time"

    influxdb2 "github.com/influxdata/influxdb-client-go/v2"
    _ "time/tzdata" // Embed the IANA Time Zone database
)

var (
    version = "0.1.0"
    debug   bool
)

type PriceData struct {
    SEKPerKWh  float64 `json:"SEK_per_kWh"`
    TimeStart  string  `json:"time_start"`
}

type Response []PriceData

func doEvery(d time.Duration, f func(time.Time)) {
    for x := range time.Tick(d) {
        f(x)
    }
}

func pushToInflux(t time.Time) {
    priceArea := os.Getenv("PRICE_AREA")
    if priceArea == "" {
        fmt.Println("Error: PRICE_AREA environment variable is not set")
        os.Exit(1)
    }

    now := time.Now()
    today := now.Format("2006/01-02")
    tomorrow := now.AddDate(0, 0, 1).Format("2006/01-02")

    var urls []string
    baseURL := "https://www.elprisetjustnu.se/api/v1/prices/"
    urls = append(urls, baseURL+today+"_"+priceArea+".json")
    if now.Hour() >= 14 && now.Minute() >= 30 {
        urls = append(urls, baseURL+tomorrow+"_"+priceArea+".json")
    }

    client := &http.Client{}
    for _, url := range urls {
        if debug {
            fmt.Println("Fetching URL:", url)
        }

        req, err := http.NewRequest("GET", url, nil)
        if err != nil {
            fmt.Println("Error creating request:", err)
            continue
        }
        req.Header.Set("User-Agent", fmt.Sprintf("vattenfall-to-influxdb/%s (+https://github.com/rvoitenko/vattenfall-to-influxdb)", version))

        resp, err := client.Do(req)
        if err != nil {
            fmt.Println("Error on request:", err)
            continue
        }
        defer resp.Body.Close()

        if resp.StatusCode != http.StatusOK {
            fmt.Printf("Request to %s failed with status code: %d\n", url, resp.StatusCode)
            bodyBytes, _ := ioutil.ReadAll(resp.Body)
            fmt.Println("Response body:", string(bodyBytes))
            continue
        }

        body, err := ioutil.ReadAll(resp.Body)
        if err != nil {
            fmt.Println("Error reading response body:", err)
            continue
        }

        var result Response
        if err := json.Unmarshal(body, &result); err != nil {
            fmt.Println("Can not unmarshal JSON:", err)
            continue
        }

        if debug {
            fmt.Println("Received data:", result)
        }

        influxClient := influxdb2.NewClient(os.Getenv("INFLUXDB_URL"), os.Getenv("INFLUXDB_TOKEN"))
        writeAPI := influxClient.WriteAPIBlocking(os.Getenv("INFLUXDB_ORG"), os.Getenv("INFLUXDB_BUCKET"))
        for _, data := range result {
            startDate, err := time.Parse(time.RFC3339, data.TimeStart)
            if err != nil {
                fmt.Println("Error parsing start time:", err)
                continue
            }

            point := influxdb2.NewPoint("current",
                map[string]string{"unit": "price"},
                map[string]interface{}{"last": data.SEKPerKWh},
                startDate)

            if debug {
                fmt.Printf("Data to be written to InfluxDB: Time: %s, SEK_per_kWh: %f\n", data.TimeStart, data.SEKPerKWh)
            }

            writeAPI.WritePoint(context.Background(), point)
        }
        influxClient.Close()
    }
    if debug {
        fmt.Printf("%v: Tick\n", t)
    }
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
