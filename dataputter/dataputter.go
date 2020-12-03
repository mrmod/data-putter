// DataPutter
//
// Handles requests from remote systems to store bytes,
// which are associated with an ObjectID, to disk.
package dataputter

import (
	"context"
	"fmt"
	"net"
	"os"
	"strings"
	"time"
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

func ServeTicketBytes(c net.Conn, client WriteNodeClient) error {
	defer c.Close()
	ticketIDBytes := make([]byte, 8)

	if l, err := c.Read(ticketIDBytes); err != nil || l != 8 {
		fmt.Printf("Failed to get ticketID from %d bytes: %v\n", l, err)
		return err
	}

	ticketID := string(ticketIDBytes)
	readRequest := &NodeReadRequest{
		TicketId: ticketID,
	}
	fmt.Printf("ServeTicketBytes for %s\n", ticketID)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)

	response, err := client.Read(ctx, readRequest)
	defer cancel()

	if response.Status != 0 {
		fmt.Printf("Failed to read ticket %s\n", response.TicketId)
	}
	return err
}

// Handles confirmations from a data putter node that it has deleted
// the bytes associated with a ticket
// * Has Datastore access
func DeleteObjectReferences(objectID, ticketID string) (Ticket, error) {
	ticket, err := GetTicketMetadata(ticketID)
	if err != nil {
		return ticket, err
	}

	err = DeleteTicket(objectID, ticketID)
	if err != nil {
		return ticket, err
	}

	remainingTickets, err := ReduceTicketCounter(objectID)
	if err != nil {
		return ticket, err
	}

	if remainingTickets > 0 {
		fmt.Printf("%d more tickets remain for %s\n", remainingTickets, objectID)
		return ticket, nil
	}

	fmt.Printf("All tickets referenced by %s have been deleted\n", objectID)
	return ticket, DeleteObjectReference(objectID)
}

// Delete an objects tickets from DataPutter Nodes
// * Delete bytes (Ticket bytes) from Putter Nodes
// * Delete Ticket references
// * Delete Object reference
// * Has Datastore access
func DeleteObject(objectID string) ([]Ticket, error) {
	tickets, err := GetObjectTickets(objectID)
	deletedTickets := []Ticket{}

	if err != nil {
		fmt.Printf("Delete object failed to get tickets for %s: %v\n", objectID, err)
		return deletedTickets, err
	}
	// Remove the bytes from disk
	// Discover ticket information in a Datastore connected role
	for ticketIndex, ticketID := range tickets {
		nodeID, err := GetTicketNode(ticketID)
		if err != nil {
			return deletedTickets, fmt.Errorf("Unable to find node for ticket %s: %v\n", ticketID, err)
		} else {
			fmt.Printf("DEBUG: deleting ticket [%d/%d] %s from node %s\n",
				ticketIndex, len(tickets), ticketID, nodeID,
			)
		}
		deleteRequest := &NodeDeleteRequest{
			ObjectId: objectID,
			TicketId: ticketID,
			NodeId:   nodeID,
		}

		// TODO: Should use service lookup to find nodes
		// during each segment
		fmt.Println("Creating nodeClient")
		nodeClient, err := NewClient("127.0.0.1:5002")
		fmt.Println("Created nodeClient")
		if err != nil {
			fmt.Printf("Unable to create NodeClient: %v\n", err)
			return deletedTickets, err
		}
		defer nodeClient.Close()

		// Send Delete to NodeClient
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		res, err := nodeClient.Delete(ctx, deleteRequest)
		defer cancel()

		if err != nil || res.Status != 0 {
			fmt.Printf("Error deleting ticket bytes for %s of %s: %v\n",
				deleteRequest.TicketId,
				deleteRequest.ObjectId,
				err)
			return deletedTickets, err
		}
		ticket, err := DeleteObjectReferences(
			deleteRequest.ObjectId,
			deleteRequest.TicketId,
		)
		if err != nil {
			return deletedTickets, err
		}
		deletedTickets = append(deletedTickets, ticket)
	}

	return deletedTickets, nil
}
