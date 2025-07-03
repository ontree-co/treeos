package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "setup-dirs" {
		fmt.Println("Running directory setup...")
		return
	}

	fmt.Println("Starting server...")
}