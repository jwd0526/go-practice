package main

import (
	"context"
	"encoding/csv"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/chromedp/chromedp"
)

type Listing struct {
	Title string
	Price string
	Mileage string
	City string
	Distance string
}

func compareListings(previous, current []Listing) ([]Listing, []Listing) {
	var newListings, removedListings []Listing
	
	// Find new listings
	for _, curr := range current {
		found := false
		for _, prev := range previous {
			if curr.Title == prev.Title && curr.Price == prev.Price && curr.Mileage == prev.Mileage {
				found = true
				break
			}
		}
		if !found {
			newListings = append(newListings, curr)
		}
	}
	
	// Find removed listings
	for _, prev := range previous {
		found := false
		for _, curr := range current {
			if prev.Title == curr.Title && prev.Price == curr.Price && prev.Mileage == curr.Mileage {
				found = true
				break
			}
		}
		if !found {
			removedListings = append(removedListings, prev)
		}
	}
	
	return newListings, removedListings
}

func main() {
	// Set up Chrome options for CI environments
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-first-run", true),
		chromedp.Flag("no-default-browser-check", true),
		chromedp.Flag("disable-background-timer-throttling", true),
		chromedp.Flag("disable-renderer-backgrounding", true),
		chromedp.Flag("disable-backgrounding-occluded-windows", true),
	)

	allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancel()

	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	// Set a timeout
	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// URL to scrape
	url := "https://www.autotempest.com/results?make=toyota&model=camry&zip=30605&radius=500&maxprice=20000&minyear=2020&maxyear=2025&maxmiles=70000&transmission=auto"

	// Variables to store our results
	var titles, prices, mileages, cities, distances []string

	// Run tasks
	err := chromedp.Run(ctx,
		// Navigate to the page
		chromedp.Navigate(url),
		
		// Wait for the page to load
		chromedp.Sleep(2*time.Second),
		
		// Wait for listings to appear in DOM
		chromedp.WaitVisible(`a.listing-link.source-link`),
		
		// Extract each field separately
		chromedp.Evaluate(`Array.from(document.querySelectorAll('a.listing-link.source-link')).map(el => el.innerText.trim())`, &titles),
		chromedp.Evaluate(`Array.from(document.querySelectorAll('div.label--price')).map(el => el.innerText.trim())`, &prices),
		chromedp.Evaluate(`Array.from(document.querySelectorAll('span.mileage')).map(el => el.innerText.trim())`, &mileages),
		chromedp.Evaluate(`Array.from(document.querySelectorAll('span.city')).map(el => el.innerText.trim())`, &cities),
		chromedp.Evaluate(`Array.from(document.querySelectorAll('span.distance')).map(el => el.innerText.trim())`, &distances),
	)

	if err != nil {
		log.Fatal(err)
	}

	// Read existing CSV file to get previous listings
	var previousListings []Listing
	if file, err := os.Open("listings.csv"); err == nil {
		defer file.Close()
		reader := csv.NewReader(file)
		records, err := reader.ReadAll()
		if err == nil && len(records) > 1 { // Skip header
			for _, record := range records[1:] {
				if len(record) >= 5 {
					previousListings = append(previousListings, Listing{
						Title:    record[0],
						Price:    record[1],
						Mileage:  record[2],
						City:     record[3],
						Distance: record[4],
					})
				}
			}
		}
	}

	// Create Listing structs from separate slices
	var listings []Listing
	maxLen := len(titles)
	if len(prices) > maxLen {
		maxLen = len(prices)
	}
	if len(mileages) > maxLen {
		maxLen = len(mileages)
	}
	if len(cities) > maxLen {
		maxLen = len(cities)
	}
	if len(distances) > maxLen {
		maxLen = len(distances)
	}

	for i := 0; i < maxLen; i++ {
		listing := Listing{}
		if i < len(titles) {
			listing.Title = titles[i]
		}
		if i < len(prices) {
			listing.Price = prices[i]
		}
		if i < len(mileages) {
			listing.Mileage = mileages[i]
		}
		if i < len(cities) {
			listing.City = cities[i]
		}
		if i < len(distances) {
			listing.Distance = distances[i]
		}
		listings = append(listings, listing)
	}

	// Create/overwrite CSV file
	file, err := os.Create("listings.csv")
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write header
	header := []string{"Title", "Price", "Mileage", "City", "Distance"}
	if err := writer.Write(header); err != nil {
		log.Fatal(err)
	}

	// Write listings to CSV
	for _, listing := range listings {
		record := []string{listing.Title, listing.Price, listing.Mileage, listing.City, listing.Distance}
		if err := writer.Write(record); err != nil {
			log.Fatal(err)
		}
	}

	// Compare listings and write changes to text file
	newListings, removedListings := compareListings(previousListings, listings)
	
	if len(newListings) > 0 || len(removedListings) > 0 {
		changesFile, err := os.Create("listing_changes.txt")
		if err != nil {
			log.Fatal(err)
		}
		defer changesFile.Close()

		fmt.Fprintf(changesFile, "Listing Changes - %s\n", time.Now().Format("2006-01-02 15:04:05"))
		fmt.Fprintf(changesFile, "=====================================\n\n")

		if len(newListings) > 0 {
			fmt.Fprintf(changesFile, "NEW LISTINGS (%d):\n", len(newListings))
			fmt.Fprintf(changesFile, "-------------------\n")
			for i, listing := range newListings {
				fmt.Fprintf(changesFile, "%d. %s | %s | %s | %s | %s\n", 
					i+1, listing.Title, listing.Price, listing.Mileage, listing.City, listing.Distance)
			}
			fmt.Fprintf(changesFile, "\n")
		}

		if len(removedListings) > 0 {
			fmt.Fprintf(changesFile, "REMOVED LISTINGS (%d):\n", len(removedListings))
			fmt.Fprintf(changesFile, "---------------------\n")
			for i, listing := range removedListings {
				fmt.Fprintf(changesFile, "%d. %s | %s | %s | %s | %s\n", 
					i+1, listing.Title, listing.Price, listing.Mileage, listing.City, listing.Distance)
			}
		}

		fmt.Printf("Found %d listings, %d new, %d removed. Changes saved to listing_changes.txt\n", 
			len(listings), len(newListings), len(removedListings))
	} else {
		fmt.Printf("Found %d listings, no changes detected\n", len(listings))
	}
}