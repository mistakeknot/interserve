package main

import (
	"fmt"
	"os"

	"github.com/mark3labs/mcp-go/server"
	"github.com/mistakeknot/interserve/internal/tools"
)

func main() {
	s := server.NewMCPServer(
		"interserve",
		"0.1.0",
		server.WithToolCapabilities(true),
	)

	dispatchPath := os.Getenv("INTERSERVE_DISPATCH_PATH")
	if dispatchPath == "" {
		dispatchPath = "/root/projects/Interverse/hub/clavain/scripts/dispatch.sh"
	}

	if info, err := os.Stat(dispatchPath); err != nil {
		fmt.Fprintf(os.Stderr, "interserve-mcp: dispatch path %q: %v\n", dispatchPath, err)
		os.Exit(1)
	} else if info.IsDir() {
		fmt.Fprintf(os.Stderr, "interserve-mcp: dispatch path %q is a directory, expected a file\n", dispatchPath)
		os.Exit(1)
	}

	tools.RegisterAll(s, dispatchPath)

	if err := server.ServeStdio(s); err != nil {
		fmt.Fprintf(os.Stderr, "interserve-mcp: %v\n", err)
		os.Exit(1)
	}
}
