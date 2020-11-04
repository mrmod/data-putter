// Router
//
// The router is responsible for listening for an entire
// file of bytes from an object owner. It reads at least 1458 bytes into memory.
//
// Bytes are ready in 1450-byte chunks preceded by an 8-byte content length int.
// Each time a chunk is ready, it is assigned a ticket and a checksum (TODO) is done on the bytes.
//
// After the ticket, a WriteTicket and checksum are created, they are sent along with
// their data bytes to a PutterNode so it can write them to disk somewhere.
package dataputter

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"sync"
)

// PutterRequest : Serializable request transmissible to any
// DataPutter node
type PutterRequest interface {
	Write() error
	GetTicketID() []byte
	String() string
}

// PutterResponse : Sent by Putter when request has finished
type PutterResponse struct {
	ObjectID string
	TicketID []byte
	NodeID   string
	Status   int
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
		// Handle a request to store a file/bunch-of-bytes somewhere
		go putterRequestHandler(conn, putterRequests)
	}
}

// PutterResponseServer : Listens for TicketID and Status responses sent from PutterNodes
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

// putterRepsonseHandler : Handles responses sent from PutterNodes in response to
// a PutRequest.
// When a WriteTicket request completes the response handler takes the
// message it receives from the DataPutterNode and updates observers of
// putterResponses
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
		putterResponse := PutterResponse{
			TicketID: dataStream[0:8],
			Status:   int(dataStream[8]),
			NodeID:   c.RemoteAddr().String(),
		}

		fmt.Printf("PutterResponse from Node %s for Ticket %s: %d\n",
			putterResponse.NodeID,
			string(putterResponse.TicketID),
			putterResponse.Status,
		)

		waitForObjectID := make(chan string, 1)

		spinOnObjectCreation := func(tid string, waiter chan string) {
			defer close(waitForObjectID)
			// A watcher might be more elegant
			for {
				objectID, err := GetTicketObject(tid)
				if err != nil {
					fmt.Printf("Unable to find objectID for ticket %s: %v\n", tid, err)
				} else {
					// Ignore the keypath
					_, err = TouchWriteCounter(objectID)
					if err != nil {
						fmt.Printf("\tFailed to update write counter on %s\n", objectID)
					} else {
						fmt.Printf("\tUpdated Object writeCounter of %s because of ticket %s\n", objectID, tid)
					}
					waiter <- objectID
					break
				}
			}
		}

		go spinOnObjectCreation(string(putterResponse.TicketID), waitForObjectID)

		putterResponse.ObjectID = <-waitForObjectID
		fmt.Printf("\tObject %s exists\n", putterResponse.ObjectID)
		putterResponses <- putterResponse
		return nil
	}
	return nil
}

// putterRequestHandler : Handles a single TCP connection creating an
// ObjectID for the file and then WriteTickets for each byte region.
// When it has written all the bytes sent, it will reply with the 8 Byte
// ObjectID it has assigned.
func putterRequestHandler(c net.Conn, putterRequests chan PutterRequest) error {
	defer c.Close()

	// Grant a new ObjectID for this TCP connection / file
	objectID := NextObjectID()
	fmt.Printf("Handling router connection for Object: %s\n", string(objectID))

	// [8B size][1450B data]
	// Limits requests to 16 GB
	// ContentLength must be bigEndian
	contentLenBuf := make([]byte, 8)
	if _, err := c.Read(contentLenBuf); err != nil {
		fmt.Printf("Unable to get the content lenght of Object %s: %s\n", string(objectID), err)
		return err
	}
	fmt.Printf("Read contentLen as %s\n", string(contentLenBuf))
	// err := binary.Read(contentLenBuf, binary.BigEndian, &contentLength)
	contentLength := int64(binary.BigEndian.Uint64(contentLenBuf))

	fmt.Printf("Opening %d bytes of content from Object %s\n", contentLength, string(objectID))
	var n int
	var err error
	var bytesRead = int64(0)

	var writeInProgress sync.WaitGroup
	writeInProgress.Add(1)

	// Consumers waiting on Object write to complete
	writeWaiters := make(chan CounterEvent, 2)

	// Write regions of bytes for this object
	for {
		fmt.Printf("-- -- --\n")
		dataStream := make([]byte, 1450)
		ticketID := NextTicketID()
		fmt.Printf("Trying to read bytes from Object %s stream\n", string(objectID))
		n, err = c.Read(dataStream)
		fmt.Printf("\tRead %d bytes from Object %s stream\n", n, string(objectID))
		if err != nil && err != io.EOF {
			fmt.Printf("Error reading bytes from %d onward: %v\n", bytesRead, err)
			return err
		}

		if err == io.EOF {
			fmt.Printf("\tEOF Read %d bytes of object %s\n", bytesRead, string(objectID))
			break
		}

		putRequest := ObjectWriteTicket{
			ObjectID:  string(objectID),
			ByteStart: bytesRead,
			ByteEnd:   bytesRead + int64(n),
			ByteCount: int64(n),
			WriteTicket: WriteTicket{
				TicketID: ticketID,
				Checksum: make([]byte, 8),
				Data:     dataStream,
			},
		}
		// Create a new object
		if err := CreateObject(putRequest.ObjectID, string(putRequest.TicketID)); err != nil {
			fmt.Printf("Unable to create Object %s: %v\n", putRequest.ObjectID, err)
			return err
		}

		// Create a new ticket for each put request
		if err := CreateTicket(
			string(putRequest.WriteTicket.TicketID),
			putRequest.ObjectID,
			"TARGET_PUTTER_NODE_UNKNOWN",
			putRequest.ByteStart,
			putRequest.ByteEnd,
			putRequest.ByteCount,
		); err != nil {
			fmt.Printf("Unable to create Ticket %s on Object %s: %v\n",
				string(putRequest.TicketID),
				putRequest.ObjectID,
				err,
			)

			return err
		}
		// Send putter request to writer
		if n > 0 {
			keyPath, err := TouchTicketCounter(string(objectID))

			// TODO: Start ticketCounter writeCounter monitor exiting when both are equal
			if err != nil {
				fmt.Printf("Unable to update ticket counter at %s: %v\n", keyPath, err)
				return err
			}
			// Create a ticket
			if bytesRead == int64(0) {
				fmt.Printf("Setting object %s writing to 'Writing'\n", string(objectID))
				if err := SetObjectStatus(putRequest.ObjectID, ObjectStatus[ObjectWriting]); err != nil {
					fmt.Printf("Unable to move Object %s into writing status: %v\n", putRequest.ObjectID, err)
					return err
				}
				fmt.Printf("Set Object %s status to %s\n", putRequest.ObjectID, ObjectStatus[ObjectWriting])
			}

			bytesRead += int64(n)
			fmt.Printf("Read %d of %d bytes\n", n, contentLength)
			putterRequests <- putRequest

			if putRequest.ByteEnd == contentLength {
				fmt.Printf("\tRead all %d bytes of %d for Object %s\n",
					putRequest.ByteEnd,
					contentLength,
					string(objectID),
				)
				break
			}
		} else {
			fmt.Printf("Unexpected 0 byte read\n")
		}
	}
	// At this point we have access to the length of the object in bytes
	// Wait for the file to be written
	fmt.Printf("Waiting for Object %s to be written\n", string(objectID))
	go spinWhileObjectWriting(string(objectID), writeWaiters, &writeInProgress)
	go WatchCounter("/objects/"+string(objectID)+"/writeCounter", writeWaiters)
	fmt.Printf("Waiting for WatchCounter to release inProgress for %s\n", string(objectID))
	writeInProgress.Wait()
	// close(writeWaiters)
	SetObjectByteSize(string(objectID), bytesRead)
	fmt.Printf("Persisted all %d bytes of Object %s\n", bytesRead, string(objectID))
	// Send the created objectID to the client
	n, err = c.Write(objectID)
	if err != nil {
		fmt.Printf("Failed to notify client [%d]objectID %s was committed successfully\n", n, objectID)
		return err
	}
	fmt.Printf("OK %d byte of [%d]objectID %s write committed\n", bytesRead, n, objectID)

	return c.Close()
}

func spinWhileObjectWriting(objectID string, countEvents chan CounterEvent, wg *sync.WaitGroup) {

	fmt.Printf("Wait on object write to complete")
	for countEvent := range countEvents {
		writeCounter, wcErr := GetWriteCounterValue(objectID)
		ticketCounter, tcErr := GetTicketCounterValue(objectID)

		if wcErr == nil && tcErr == nil {
			fmt.Printf("Comparing T: %d with W %d vs CE %s :: %d\n",
				ticketCounter,
				writeCounter,
				countEvent.KeyPath,
				countEvent.Value,
			)
			if writeCounter == ticketCounter && ticketCounter > 0 {
				wg.Done()
				return
			}
		} else {
			fmt.Printf("Counter event errors; %v and %v\n", wcErr, tcErr)
		}
	}
}
