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
	intake := make(chan dataputter.WriteTicket, 4)
	defer close(intake)

	if len(os.Args) <= 1 {
		fmt.Println("USAGE: app [putter|loopback|router]")
		os.Exit(1)
	}

	startupMode := os.Args[1]

	switch startupMode {
	// Byte Putter Router server
	case "router":
		go func(work chan dataputter.WriteTicket) {
			for wt := range work {
				fmt.Printf("\tRouter ticket %d %s\n", len(wt.TicketID), string(wt.TicketID))
				fmt.Printf("\tOutpath: %s\n", dataputter.ObjectPathString(string(wt.TicketID)))
				if err := wt.Write(); err != nil {
					fmt.Printf("Error writing ticket %s: %v\n", string(wt.TicketID), err)
				}
			}
		}(intake)
		dataputter.RouterServer(5001, intake)
	// Byte Putter server
	case "putter":
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
