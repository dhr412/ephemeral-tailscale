package main

import (
	"fmt"
	"os"
)

func printUsage() {
	fmt.Printf("Usage: %s [<mode>]\n\n"+
		"Modes:\n"+
		"  embedded  Use tsnet TCP proxy (default if no mode is provided)\n"+
		"  client    Use Tailscale client\n\n"+
		"Help:\n"+
		"  -h, --help, help   Show this help message\n", os.Args[0])
}

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "-h", "--help", "-help", "help":
			printUsage()
			os.Exit(0)

		case "embedded":
			runEmbedded()

		case "client":
			runCliented()

		default:
			fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", os.Args[1])
			printUsage()
			os.Exit(1)
		}
	}

	runEmbedded()
}
