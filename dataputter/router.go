// Router
//
// The router is responsible for listening for an entire
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
// a PutRequest
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

		fmt.Printf("Read response from Node %s for Ticket %s: %d\n",
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
						fmt.Printf("\tUpdated writeCounter of %s because of %s\n", objectID, tid)
					}
					waiter <- objectID
					break
				}
			}
		}

		go spinOnObjectCreation(string(putterResponse.TicketID), waitForObjectID)

		putterResponse.ObjectID = <-waitForObjectID
		fmt.Printf("\tObject created %s\n", putterResponse.ObjectID)
		putterResponses <- putterResponse
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
	var byteCount = int64(0)
	var ticketID []byte

	var writeInProgress sync.WaitGroup
	writeInProgress.Add(1)

	writeWaiters := make(chan CounterEvent, 2)

	go spinWhileObjectWriting(string(objectID), writeWaiters, &writeInProgress)
	// Write regions of bytes for this object
	for {
		ticketID = TicketGenerator(ticketID)
		n, err = c.Read(dataStream)
		if err != nil && err != io.EOF {
			fmt.Printf("Error reading bytes from %d onward: %v\n", byteCount, err)
			return err
		}

		// How do we know when all tickets are saved?
		if err == io.EOF {
			break
		}

		putRequest := ObjectWriteTicket{
			ObjectID:  string(objectID),
			ByteStart: byteCount,
			ByteEnd:   byteCount + int64(n),
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

		/*
			How to know when the object has all been written?

			One way is to count all the ticket creations and fork a watcher for each ETCD /tickets/$ticketID/status key.
			When the status is saved, reduce the counter by 1. The counter might become zero more than once depending on
			when in the for { connection } loop the counter is adjusted.

			Another way is to maintain a counter in /objects/$objectID/ticketCounter and fork a watch for each
			ETCD /tickets/$ticketID/status key. Additionally, etcd://objects/$objectID/writeCounter should be created.
			Finally, a watcher should be setup on /writeCounter.
			When a etcd://tickets/$ticketID/status key becomes saved, a counter at etcd://objects/$objectID/writeCounter
			should be incremented.
			When etcd://objects/$objectID/writeCounter matches the value in ./ticketCounter, the write is complete.

			With the second way, there's a chance to do replication without making things complicated later. Any replicated object
			is just an object so tacking on a etcd://objects/$objectID/replicatedFromObject or some such thing would
			work fine.

		*/

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
			if byteCount == int64(0) {
				fmt.Printf("Setting object %s writing to 'Writing'\n", string(objectID))
				if err := SetObjectStatus(putRequest.ObjectID, ObjectStatus[ObjectWriting]); err != nil {
					fmt.Printf("Unable to move Object %s into writing status: %v\n", putRequest.ObjectID, err)
					return err
				}
				fmt.Printf("Set Object %s status to %s\n", putRequest.ObjectID, ObjectStatus[ObjectWriting])
			}

			byteCount += int64(n)
			fmt.Printf("Read %d of %d bytes\n", n, byteCount)
			putterRequests <- putRequest
		}
	}
	// At this point we have access to the length of the object in bytes
	// Wait for the file to be written
	fmt.Printf("Waiting for Object %s to be written\n", string(objectID))
	go WatchCounter("/objects/"+string(objectID)+"/writeCounter", writeWaiters)
	writeInProgress.Wait()
	close(writeWaiters)
	fmt.Printf("Wrote Object %s to enough nodes\n", string(objectID))
	return nil
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
