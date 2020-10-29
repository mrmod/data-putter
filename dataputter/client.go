// DataPutter is a very simplistic object store for people with private, properly configured,
// networks.
//
// Client
//
// Clients  or DataPutterClient send bytes and a ticket
// to a DataPutter (server) node to have things written to disk
package dataputter

import (
	"fmt"
	"net"
)

// PutterNode of a DataPutter which can write bytes to disk
type PutterNode struct {
	Host string
	Port int
}

// String Satisfies Node interface
func (p PutterNode) String() string {
	return fmt.Sprintf("%s:%d", p.Host, p.Port)
}

// Node Generic access to a node
type Node interface {
	String() string
}

// PutToTarget Submit a write ticket to a writer node
func PutToTarget(wt WriteTicket, putter Node) (WriteTicket, error) {

	c, err := net.Dial("tcp", putter.String())
	if err != nil {
		return wt, err
	}
	defer c.Close()

	n, err := c.Write(wt.TicketID)
	if err != nil {
		return wt, err
	}
	fmt.Printf("Sent %d byte TicketID\n", n)

	n, err = c.Write(wt.Checksum)
	fmt.Printf("Sent %d byte Checksum\n", n)
	if err != nil {
		return wt, err
	}

	n, err = c.Write(wt.Data)
	if err != nil {
		return wt, err
	}
	fmt.Printf("Sent %d bytes of data\n", n)
	return wt, nil
}
