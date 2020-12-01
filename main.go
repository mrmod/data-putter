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
	"net"
	"os"

	"github.com/mrmod/data-putter/dataputter"
	"google.golang.org/grpc"
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
		intake := make(chan dataputter.PutterRequest, 4)
		putterResponses := make(chan dataputter.PutterResponse, 4)
		defer close(intake)
		defer close(putterResponses)

		// Simulate DataPutter node
		go func(work chan dataputter.PutterRequest) {
			for putterRequest := range work {
				if err := putterRequest.Write(); err != nil {
					fmt.Printf("Error writing ticket %s: %v\n", putterRequest, err)
				} else {
					// Simulation is assumed to run on the same host
					routerNodeAddress := "127.0.0.1:5002"
					fmt.Printf("Preparing response to Ticket %s\n",
						string(putterRequest.GetTicketID()),
					)
					dataputter.SendPutResponse(
						routerNodeAddress,
						putterRequest.GetTicketID(),
						dataputter.TicketSaved,
					)
				}
			}
		}(intake)

		// Listen for PutRequest responses from the Simulated DataPutter node
		// ResponseHandler for PutRequests(WriteTicket) sent to DataPutter nodes
		go func(responses chan dataputter.PutterResponse) {
			for response := range responses {
				fmt.Printf("Ticket %s PutterResponse status: %d\n", string(response.TicketID), response.Status)
				if err := dataputter.SetTicketStatus(
					string(response.TicketID),
					dataputter.TicketStatus[response.Status],
				); err != nil {
					fmt.Printf("Failed to move Ticket %s to status %d: %v\n",
						string(response.TicketID),
						response.Status,
						err,
					)
				} else {
					fmt.Printf("\tCommitted Ticket %s status %s to datatore\n",
						string(response.TicketID),
						dataputter.TicketStatus[response.Status],
					)
				}
			}
		}(putterResponses)

		// Listen for requests to write or delete tickets
		// WriteNode on :5002
		writeNodeAddress := "127.0.0.1:5002"
		writeNodeServer := grpc.NewServer()
		writeNode := &dataputter.WriteNodeService{}
		dataputter.RegisterWriteNodeServer(writeNodeServer, writeNode)
		// WriteNode.Serve()
		go func() {
			tcpListener, err := net.Listen("tcp", writeNodeAddress)
			if err != nil {
				fmt.Printf("Unable to create tcp/5002 for WriteNode: %v\n", err)
				return
			}
			if err := writeNodeServer.Serve(tcpListener); err != nil {
				fmt.Printf("Error starting WriteNode on 5002: %v\n", err)
				return
			}
		}()
		writeNodeConn, err := grpc.Dial(writeNodeAddress, grpc.WithInsecure(), grpc.WithBlock())
		if err != nil {
			fmt.Printf("Failed to create WriteNodeClient: %v\n", err)
			return
		}
		defer writeNodeConn.Close()
		writeClient := dataputter.NewWriteNodeClient(writeNodeConn)

		// Listen for Object read requests [Router]
		go dataputter.ObjectRequestServer(5004, writeClient)
		// Listen for Ticket read requests [PutterNode]
		go dataputter.TicketRequestServer(5005, writeClient)
		// Listen for inbound files [Router]
		// TODO: Router is writeNode client
		dataputter.RunRouterServer(5001, writeClient)

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
