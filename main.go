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
		go StartWriteNode(nodeConfig.Host, nodeConfig.Port)
	}
	fmt.Printf("Started %d Write Nodes\n", len(config.Nodes))
	// Listen for inbound files [Router]
	go StartReadServer(5004)
	StartRouter(config)

}

// StartWriteNode On this machine
func StartWriteNode(bind string, port int) {
	nodeService := dataputter.NewWriteNodeService(bind, port)
	nodeService.Serve()
}

// StartReadServer On this machine to listen for read requests
func StartReadServer(port int) {
	fmt.Printf("Starting read server on %d\n", port)
	dataputter.ObjectServer(port)
}

// StartRouter On this machine
func StartRouter(config dataputter.RouterConfig) {
	fmt.Printf("Starting router on %d\n", config.Port)
	dataputter.RunRouterServer(config)
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
	fmt.Printf("Starting in %s mode\n", startupMode)
	switch startupMode {
	case "standAlone":
		StandAlone(config)
	case "router":
		go StartReadServer(5004)
		StartRouter(config)
	case "writeNode":
		StartWriteNode("0.0.0.0", 5002)
	default:
		showUsage()
	}

	fmt.Println("Shutting down...")

	return
}
