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

	"github.com/mrmod/data-putter/dataputter"
)

// Receives dataputter.WriteTickets and writes them serially to disk
func writeTicketHandler(work chan dataputter.WriteTicket) {
	for wt := range work {
		fmt.Printf("Received %d bytes of work: %s\n", len(wt.Data), string(wt.TicketID))
		// Write to disk
		err := dataputter.StoreBytes(wt)
		if err != nil {
			fmt.Printf("Error writing ticket %s with %d bytes of data: %v\n",
				string(wt.TicketID), len(wt.Data), err)
		}
	}
}

// [8B TicketID][8B Checksum][nB Data]
// Data should be 1458 Bytes for best results
func parseTicketRequest(b []byte) dataputter.WriteTicket {
	return dataputter.WriteTicket{
		b[0:8],
		b[8:16],
		b[16:],
	}
}

// handle connections for clients looking to put bytes
func handleConnection(c net.Conn, intake chan dataputter.WriteTicket) {
	defer c.Close()
	fmt.Println("Handling connection")
	ticketRequest := make([]byte, 1500)
	// Read until nil
	n, err := c.Read(ticketRequest)
	if n < 16 {
		err = fmt.Errorf("Too few bytes %d", n)
	}
	if err != nil {
		fmt.Printf("Error reading from connection: %v\n", err)
	} else {
		fmt.Printf("Read %d Bytes: %v\n", n, string(ticketRequest))
		writeTicket := parseTicketRequest(ticketRequest)
		fmt.Printf("Ticket: %s\n", writeTicket)
		intake <- writeTicket
	}
}

// create a TCP server to listen for dataputter.WriteTickets
func createServer(port string, intake chan dataputter.WriteTicket) error {
	s, err := net.Listen("tcp", port)
	if err != nil {
		return err
	}

	for {
		conn, err := s.Accept()
		if err != nil {
			fmt.Printf("Error in connection: %v\n", err)
			continue
		}
		go handleConnection(conn, intake)
	}
}

func main() {
	fmt.Println("Hello old friend")
	intake := make(chan dataputter.WriteTicket, 4)
	defer close(intake)

	go writeTicketHandler(intake)
	if err := createServer(":5000", intake); err != nil {
		fmt.Printf("Error starting server: %v\n", err)
	}
	dataputter.Write(
		&dataputter.APIPutRequest{
			ObjectID: "someData",
			Bytes:    []byte("some test data"),
		},
	)
	return
}
