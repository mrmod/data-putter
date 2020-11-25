// DataPutter
//
// Handles requests from remote systems to store bytes,
// which are associated with an ObjectID, to disk.
package dataputter

import (
	"fmt"
	"io/ioutil"
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

// deleteBytes: Always delete bytes from filename given
// on a node storing the object
func deleteBytes(filename string) error {
	_, err := os.Stat(filename)
	if os.IsNotExist(err) {
		fmt.Printf("%s does not exist: %v\n", filename, err)
		return err
	}
	return os.Remove(filename)
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

	response := append(ticketID, byte(statusCode))
	fmt.Printf("PutterNode: Created %d-byte response using:\n", len(response))
	fmt.Printf("\t%d bytes for TicketID\n", len(ticketID))
	fmt.Printf("\t%d bytes for StatusCode\n", len([]byte{byte(statusCode)}))
	return sendTCPSimple(
		fmt.Sprintf("%s:%s", host, port),
		response,
	)
}

func ServeTicketBytes(c net.Conn) error {
	defer c.Close()

	ticketIDBytes := make([]byte, 8)

	if l, err := c.Read(ticketIDBytes); err != nil || l != 8 {
		fmt.Printf("Failed to get ticketID from %d bytes: %v\n", l, err)
		return err
	}

	ticketID := string(ticketIDBytes)
	fmt.Printf("Serving ticket %s\n", ticketID)

	filename := ObjectPathString(string(ticketID)) + "/obj"

	ticketBytes, err := ioutil.ReadFile(filename)

	if err != nil {
		return err
	}
	n, err := c.Write(ticketBytes)
	if err != nil {
		return err
	}
	fmt.Printf("\tServed %d bytes for ticket %s\n", n, ticketID)
	return nil
}

// Handles confirmations from a data putter node that it has deleted
// the bytes associated with a ticket
// * Has Datastore access
func DeleteReferenceHandler(deleteConfirmations chan DeleteTicketConfirmation) {
	for confirmation := range deleteConfirmations {
		if !confirmation.Success {
			fmt.Printf("Failed to delete ticket bytes %s/%s\n",
				confirmation.ObjectID,
				confirmation.TicketID,
			)
			continue
		}
		err := DeleteTicket(
			confirmation.ObjectID,
			confirmation.TicketID,
		)

		if err != nil {
			fmt.Printf("Failed to delete ticket reference %s/%s: %v\n",
				confirmation.ObjectID,
				confirmation.TicketID,
				err,
			)
			continue
		}

		remainingTickets, err := ReduceTicketCounter(confirmation.ObjectID)
		if err != nil {
			fmt.Printf("Failed to reduce tickets associated with %s: %v\n",
				confirmation.ObjectID,
				err,
			)
		}
		fmt.Printf("%d tickets remaining of %s\n",
			remainingTickets,
			confirmation.ObjectID,
		)

		if remainingTickets == 0 {
			fmt.Printf("All ticket references for %s deleted\n", confirmation.ObjectID)
			err = DeleteObjectReference(confirmation.ObjectID)
			if err != nil {
				fmt.Printf("Failed to delete object reference for %s: %v\n",
					confirmation.ObjectID,
					err,
				)
			} else {
				fmt.Printf("Successfully deleted %s\n", confirmation.ObjectID)
			}
		}
	}
}

// Delete an objects tickets from DataPutter Nodes
// * Delete bytes (Ticket bytes) from Putter Nodes
// * Delete Ticket references
// * Delete Object reference
// * Has Datastore access
func DeleteObject(objectID string) error {
	tickets, err := GetObjectTickets(objectID)
	if err != nil {
		fmt.Printf("Delete object failed to get tickets for %s: %v\n", objectID, err)
		return err
	}

	delRequests := make(chan DeleteTicketRequest, 2)
	delConfirmations := make(chan DeleteTicketConfirmation, 2)

	// Remove the bytes from disk
	// NOTE: This is simulation configuration. The delete ticket
	// request must be sent to a TCP connected DataPutter
	// to delete bytes.
	go DeleteTicketHandler(delRequests, delConfirmations)
	// Remove the references from the datastore
	go DeleteReferenceHandler(delConfirmations)

	// Discover ticket information in a Datastore connected role
	for ticketIndex, ticketID := range tickets {
		nodeID, err := GetTicketNode(ticketID)
		if err != nil {
			return fmt.Errorf("Unable to find node for ticket %s: %v\n", ticketID, err)
		}
		deleteTicketRequest := DeleteTicketRequest{
			ticketID,
			objectID,
			nodeID,
			int64(ticketIndex),
		}
		fmt.Printf("Created delete request for %s/%s on node %s\n",
			objectID,
			ticketID,
			nodeID,
		)
		delRequests <- deleteTicketRequest
	}

	return nil
}
