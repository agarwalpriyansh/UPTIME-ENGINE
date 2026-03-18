package main

import (
	"fmt"
	"net/http"
	"time"
)

func main(){
	targetURL := "https://dsdryfruits.in"

	fmt.Printf("Starting Day 1 Monitor...\n")
	fmt.Printf("Pinging %s\n", targetURL)
	fmt.Println("--------------------------------------------------")

	startTime := time.Now()
	resp, err := http.Get(targetURL)
	if err != nil {
		fmt.Printf("[ERROR] Failed to reach %s: %v\n", targetURL, err)
		return
	}

	defer resp.Body.Close()

	latency := time.Since(startTime)

	if resp.StatusCode == 200 {
		fmt.Printf("[SUCCESS] Checked %s - Status: %d OK - Latency: %v\n", targetURL, resp.StatusCode, latency)
	} else {
		fmt.Printf("[WARNING] Checked %s - Status: %d - Latency: %v\n", targetURL, resp.StatusCode, latency)
	}
}