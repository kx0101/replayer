package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/kx0101/replayer/internal/parser"
)

func main() {
	input := flag.String("input", "", "Input nginx log file")
	output := flag.String("output", "", "Output json file")
	format := flag.String("format", "combined", "Nginx log format (combined or common)")

	flag.Parse()

	if *input == "" || *output == "" {
		fmt.Fprintln(os.Stderr, "Error: both --input and --output are required")
		flag.Usage()
		os.Exit(1)
	}

	fmt.Printf("Converting nginx logs...\n")
	fmt.Printf("  Input:  %s\n", *input)
	fmt.Printf("  Output: %s\n", *output)
	fmt.Printf("  Format: %s\n", *format)
	fmt.Println()

	if err := parser.ConvertNginxLogs(*input, *output, *format); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\nSuccessfully converted nginx logs to %s\n", *output)
	fmt.Println("You can now replay with:")
	fmt.Printf("  ./replayer --input-file %s localhost:8080\n", *output)
}
