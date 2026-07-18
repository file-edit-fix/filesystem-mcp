package main

import (
	"fmt"
	"log"
	"os"
	"runtime"

	"github.com/mark3labs/mcp-filesystem-server/filesystemserver"
	"github.com/mark3labs/mcp-go/server"
)

// enumerateDrives returns all available drive roots (Windows only).
func enumerateDrives() []string {
	var drives []string
	if runtime.GOOS == "windows" {
		for _, d := range "ABCDEFGHIJKLMNOPQRSTUVWXYZ" {
			path := string(d) + ":\\"
			if _, err := os.Stat(path); err == nil {
				drives = append(drives, path)
			}
		}
	}
	return drives
}

func main() {
	// Parse command line arguments
	if len(os.Args) < 2 {
		fmt.Fprintf(
			os.Stderr,
			"Usage: %s <allowed-directory> [additional-directories...]\n"+
				"  Use AUTO to allow all available drives\n",
			os.Args[0],
		)
		os.Exit(1)
	}

	// Expand AUTO to all available drives
	var dirs []string
	for _, arg := range os.Args[1:] {
		if arg == "AUTO" {
			dirs = append(dirs, enumerateDrives()...)
		} else {
			dirs = append(dirs, arg)
		}
	}

	// Create and start the server
	fss, err := filesystemserver.NewFilesystemServer(dirs)
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}

	// Serve requests
	if err := server.ServeStdio(fss); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}