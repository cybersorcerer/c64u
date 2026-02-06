package main

import (
	"encoding/json"
	"fmt"

	"github.com/cybersorcerer/c64.nvim/tools/c64u/internal/api"
)

func main() {
	client := api.NewClient("10.0.0.64", 80, false)
	/*
		if client.BaseURL == "" {
			client.BaseURL = "http://10.0.0.64" // Default fallback for testing
		}
	*/

	// 1. Get Categories
	cats, err := client.GetConfigCategories()
	if err != nil {
		panic(err)
	}
	fmt.Printf("Categories: %v\n", cats)

	if len(cats) > 0 {
		// 2. Get first item details
		cat := cats[0]
		items, err := client.GetConfigCategory(cat)
		if err != nil {
			panic(err)
		}

		fmt.Printf("Items in %s: %v\n", cat, items)

		for k := range items {
			// 3. Get specific item details
			details, err := client.GetConfigItem(cat, k)
			if err != nil {
				fmt.Println("Error:", err)
				continue
			}
			bytes, _ := json.MarshalIndent(details, "", "  ")
			fmt.Printf("Details for %s/%s:\n%s\n", cat, k, string(bytes))
			break // Just one is enough
		}
	}
}
