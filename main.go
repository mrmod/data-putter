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

func main() {
	fmt.Println("Hello old friend")

	if len(os.Args) <= 1 {
		fmt.Println("USAGE: app [putter|loopback|router]")
		os.Exit(1)
	}

	startupMode := os.Args[1]

	switch startupMode {
	// Byte Putter Router server
	case "router", "singleNode", "standAlone":

		// Node services which register should be stored somewhere
		nodeService := dataputter.NewWriteNodeService("127.0.0.1", "5002")
		go nodeService.Serve()

		// Listin for object read requests [Router]
		fmt.Printf("Starting read server on 5004\n")
		go dataputter.ObjectServer(5004)

		// Listen for inbound files [Router]
		fmt.Printf("Starting router on 5001\n")
		dataputter.RunRouterServer(5001)

	// DataPutter Node
	case "putter":
		intake := make(chan dataputter.WriteTicket, 4)
		defer close(intake)

		// Handles writing ticket bytes to disk
		// When used seriously, this handler is running
		// on every node writing ticket bytes to its disks
		go dataputter.WriteTicketHandler(intake)

		if err := dataputter.CreateServer(":5000", intake); err != nil {
			fmt.Printf("Error starting server: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	// Byte Putter loopback test
	case "loopback":
		fmt.Println("Sending loopback data")
		wt, err := dataputter.PutToTarget(
			dataputter.NewWriteTicket(
				"ABCDEFGH",
				"12345678",
				[]byte("So much data to write"),
			),
			dataputter.PutterNode{
				Host: "127.0.0.1",
				Port: 5000,
			},
		)
		if err != nil {
			fmt.Printf("Error writting ticket %s: %v\n", string(wt.TicketID), err)
		}
	}

	fmt.Println("Shutting down...")

	return
}
