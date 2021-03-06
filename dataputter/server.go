// Server
//
// A Server, or PutterNode, is responsible for writing
// the data from WriteTickets to disk
package dataputter

import (
	"fmt"
	"log"
	"net"
)

// WriteTicketHandler Receives WriteTickets and writes them serially to disk
func WriteTicketHandler(work chan WriteTicket) {
	for wt := range work {
		log.Printf("Received %d bytes of work: %s\n", len(wt.Data), string(wt.TicketID))
		// Write to disk
		err := StoreBytes(wt)
		if err != nil {
			log.Printf("Error writing ticket %s with %d bytes of data: %v\n",
				string(wt.TicketID), len(wt.Data), err)
		}
	}
}

// [8B TicketID][8B Checksum][nB Data]
// Data should be 1458 Bytes for best results
func parseTicketRequest(b []byte) WriteTicket {
	return WriteTicket{
		b[0:8],
		b[8:16],
		b[16:],
	}
}

// handle connections for clients looking to put bytes
func handleConnection(c net.Conn, intake chan WriteTicket) {
	defer c.Close()
	fmt.Println("Handling connection")
	ticketRequest := make([]byte, 1500)
	// Read until nil
	n, err := c.Read(ticketRequest)
	if n < 16 {
		err = fmt.Errorf("Too few bytes %d", n)
	}
	log.Printf("Read %d byte WriteTicket\n", n)
	if err != nil {
		log.Printf("Error reading from connection: %v\n", err)
	} else {
		log.Printf("Read %d Bytes: %v\n", n, string(ticketRequest))
		writeTicket := parseTicketRequest(ticketRequest)
		log.Printf("Ticket: %s\n", writeTicket)
		intake <- writeTicket
	}
}

// CreateServer create a TCP server to listen for WriteTickets
func CreateServer(port string, intake chan WriteTicket) error {
	s, err := net.Listen("tcp", port)
	if err != nil {
		return err
	}
	log.Printf("Server up on port %s\n", port)
	for {
		conn, err := s.Accept()
		if err != nil {
			log.Printf("Error in connection: %v\n", err)
			continue
		}
		go handleConnection(conn, intake)
	}
}
