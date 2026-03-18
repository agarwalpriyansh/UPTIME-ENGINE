package main

import (
	"fmt"
	"net/http"
	"time"
)

// This is what will pass through the channel.
type PingResult struct {
	URL        string
	StatusCode int
	Latency    time.Duration
	Up         bool
}

// Notice it takes a channel ('c') as an argument instead of returning a value.
func checkURL(url string, c chan PingResult) {
	startTime := time.Now()
	resp, err := http.Get(url)
	latency := time.Since(startTime)

	if err != nil {
		// Send the failure result back through the channel pipe
		c <- PingResult{URL: url, Up: false, Latency: latency}
		return
	}
	defer resp.Body.Close()

	// Send the success result back through the channel pipe
	c <- PingResult{
		URL:        url,
		StatusCode: resp.StatusCode,
		Latency:    latency,
		Up:         resp.StatusCode == 200,
	}
}

func main() {
	urls := []string{
		"https://dsdryfruits.in",
		"https://google.com",
		"https://github.com",
		"https://stackoverflow.com/questions",
		"https://golang.org",
		"https://thiswebsitedoesnotexist.com.invalid", // This will fail
	}

	// 3. Create a channel that can send and receive PingResult structs
	resultsChannel := make(chan PingResult)

	fmt.Println("Starting Day 2 Concurrent Monitor...")
	fmt.Println("--------------------------------------------------")
	
	// Start a timer for the whole program
	programStart := time.Now()

	// 4. Fire off a background goroutine for every single URL
	// They all start sprinting at the exact same time
	for _, url := range urls {
		go checkURL(url, resultsChannel)
	}

	// 5. Wait to receive the results from the channel
	// We know exactly how many URLs we checked (len(urls)), 
	// so we wait to hear back that many times.
	for i := 0; i < len(urls); i++ {
		result := <-resultsChannel
		if result.Up {
			fmt.Printf("[SUCCESS] %s is UP (Status: %d, Latency: %v)\n", result.URL, result.StatusCode, result.Latency)
		} else {
			fmt.Printf("[DOWN]    %s is DOWN or Unreachable (Latency: %v)\n", result.URL, result.Latency)
		}
	}
	
	fmt.Println("--------------------------------------------------")
	fmt.Printf("Total execution time: %v\n", time.Since(programStart))
}