// DataPutter
//
// Handles requests from remote systems to store bytes,
// which are associated with an ObjectID, to disk.
package dataputter

import (
	"fmt"
	"net"
	"os"
	"strings"
)

// PutRequest Request with data to put somewhere
type PutRequest struct {
	ObjectID           string
	ByteStart, ByteEnd int
	Bytes              []byte
}

// putBytes: Always write bytes to filename, creating as needed
// Does not create paths
func putBytes(filename string, bytes []byte) error {
	f, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE, 0644)
	defer f.Close()
	if err != nil {
		return err
	}
	_, err = f.Write(bytes)
	if err != nil {
		return err
	}
	return nil
}

func sendTCPSimple(hostPort string, data []byte) error {
	c, err := net.Dial("tcp", hostPort)
	if err != nil {
		return err
	}

	n, err := c.Write(data)
	if err != nil {
		return err
	}

	fmt.Printf("Wrote %d byte response to %s\n", n, hostPort)
	return nil
}

// Respond to a put request sent to this datanode with a statusCode
func SendPutResponse(hostPort string, ticketID []byte, statusCode int) error {
	var host string
	port := "5002"

	addrParts := strings.Split(hostPort, ":")
	switch len(addrParts) {
	case 1:
		host = addrParts[0]
	case 2:
		host = addrParts[0]
		port = addrParts[1]
	default:
		return fmt.Errorf("Need a host:port to send responses to")
	}
	// Pad bytes into 8-byte packet
	ticketBytes := make([]byte, 8)
	for i, b := range ticketID {
		ticketBytes[i] = b
	}
	response := append(ticketBytes, byte(statusCode))
	fmt.Printf("PutterNode: Created %d-byte response using:\n", len(response))
	fmt.Printf("\t%d bytes for TicketID\n", len(ticketID))
	fmt.Printf("\t%d bytes for StatusCode\n", len([]byte{byte(statusCode)}))
	return sendTCPSimple(
		fmt.Sprintf("%s:%s", host, port),
		response,
	)
}
