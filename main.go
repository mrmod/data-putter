// Package main Allows running the DataPutter server
//
// It can be started using
//
//     go run main.go
//
// This will start a TCP listener on 127.0.0.1:5000
package main

import (
	"fmt"
	"os"

	"github.com/mrmod/data-putter/dataputter"
)

// StandAlone Operational mode for playing around on a local machine
func StandAlone(config dataputter.RouterConfig) {
	for _, nodeConfig := range config.Nodes {
		StartWriteNode(nodeConfig.Host, nodeConfig.Port)
	}
	fmt.Printf("Started %d Write Nodes\n", len(config.Nodes))
	// Listen for inbound files [Router]
	StartReadServer(5004)
	StartRouter(config.Port)

}

// StartWriteNode On this machine
func StartWriteNode(bind string, port int) {
	nodeService := dataputter.NewWriteNodeService(bind, port)
	go nodeService.Serve()
}

// StartReadServer On this machine to listen for read requests
func StartReadServer(port int) {
	fmt.Printf("Starting read server on %d\n", port)
	go dataputter.ObjectServer(port)
}

// StartRouter On this machine
func StartRouter(port int) {
	fmt.Printf("Starting router on %d\n", port)
	dataputter.RunRouterServer(port)
}
func showUsage() {
	fmt.Println("USAGE: app [router|writeNode|standAlone]")
	os.Exit(1)
}
func main() {
	fmt.Println("Hello old friend")

	if len(os.Args) <= 1 {
		showUsage()
	}

	startupMode := os.Args[1]

	config, err := dataputter.LoadRouterConfig()
	if err != nil {
		if err == dataputter.ErrNoConfig {
			fmt.Printf("No configuration present, using default")
			config = dataputter.DefaultRouterConfig
		} else {
			fmt.Printf("Unexpected error loading configuration: %v\n", err)
			return
		}
	}
	switch startupMode {
	case "standAlone":
		StandAlone(config)
	case "router":
		StartRouter(config.Port)
		StartReadServer(5004)
	case "writeNode":
		StartWriteNode("0.0.0.0", 5002)
	default:
		showUsage()
	}

	fmt.Println("Shutting down...")

	return
}
