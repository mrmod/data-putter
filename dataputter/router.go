// Package dataputter The router is responsible for listening for an entire
// file of bytes from an object owner. It reads at least 1458 bytes into memory.
//
// Bytes are ready in 1458-byte chunks until a `nil` is encountered. Each time a
// chunk is ready, it is assigned a ticket and a checksum is done on the bytes.
//
// After the ticket, a WriteTicket and checksum are created, they are sent along with
// their data bytes to a PutterNode so it can write them to disk somewhere.
package dataputter

import (
	"fmt"
	"io"
	"net"
)

// PutterRequest : Serializable request transmissible to any
// DataPutter node
type PutterRequest interface {
	Write() error
	String() string
}

// PutterResponse : Sent by Putter when request has finished
type PutterResponse struct {
	ObjectID string
	TicketID []byte
	Status   int //
}

// RouterServer Listens for bytes and creates WriteTickets
func RouterServer(port int, putterRequests chan PutterRequest) error {
	s, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return err
	}
	fmt.Printf("PutterRouter running on port %d\n", port)
	for {
		conn, err := s.Accept()
		if err != nil {
			fmt.Printf("Error in connection: %v\n", err)
			continue
		}
		go putterRequestHandler(conn, putterRequests)
	}
}

// PutterResponseServer : Listens for TicketID and Status responses
func PutterResponseServer(port int, putterResponses chan PutterResponse) error {
	s, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return err
	}
	fmt.Printf("PutterResponseServer running on port %d\n", port)
	for {
		conn, err := s.Accept()
		if err != nil {
			fmt.Printf("Error in connection: %v\n", err)
			continue
		}
		fmt.Println("Handling PutterResponse connection")
		go putterResponseHandler(conn, putterResponses)
	}
}

func putterResponseHandler(c net.Conn, putterResponses chan PutterResponse) error {
	defer c.Close()

	// [8B TicketID][1B Status]
	dataStream := make([]byte, 9)

	n, err := c.Read(dataStream)
	if err != nil {
		fmt.Printf("Error reading putterResponse: %v\n", err)
		return err
	}
	// [8B Ticket][1B Status]
	if n == 9 {
		fmt.Printf("Read response: %#v\n", dataStream)
		putterResponses <- PutterResponse{
			TicketID: dataStream[0:8],
			Status:   int(dataStream[8]),
		}
		return nil
	}
	return nil
}

var objectID []byte

// putterRequestHandler : Handles a single TCP connection creating an
// ObjectID for the file and then WriteTickets for each byte region
func putterRequestHandler(c net.Conn, putterRequests chan PutterRequest) error {
	defer c.Close()

	// Grant a new ObjectID for this TCP connection / file
	objectID = TicketGenerator(objectID)
	fmt.Printf("Handling router connection as Object: %s\n", string(objectID))

	dataStream := make([]byte, 1458)
	var err error
	var n int
	var byteCount = 0
	var ticketID []byte

	// Write regions of bytes for this object
	for {
		ticketID = TicketGenerator(ticketID)
		n, err = c.Read(dataStream)
		if err != nil && err != io.EOF {
			fmt.Printf("Error reading bytes from %d onward: %v\n", byteCount, err)
			return err
		}
		// Send putter request to writer
		if n > 0 {
			putterRequests <- ObjectWriteTicket{
				ObjectID:  string(objectID),
				ByteStart: int64(byteCount),
				ByteEnd:   int64(byteCount + n),
				ByteCount: int64(n),
				WriteTicket: WriteTicket{
					TicketID: ticketID,
					Checksum: make([]byte, 8),
					Data:     dataStream,
				},
			}

			byteCount += n
			fmt.Printf("Read %d of %d bytes\n", n, byteCount)
		}
		if err != nil && err == io.EOF {
			break
		}
	}

	return nil
}
