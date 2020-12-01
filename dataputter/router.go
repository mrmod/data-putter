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
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"sync"
	"time"
)

const (
	STANDALONE_NODE_ID         = "TARGET_PUTTER_NODE_UNKNOWN"
	DELETE_HEADER              = "0DEL0DEL"
	DEFAULT_AUTHENTICITY_TOKEN = "ABadSharedToken!"
	NodeSuccess                = 0
	NodeFailed                 = 1
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

type routerServer struct {
	UnimplementedRouterServer
}

func (s *routerServer) CreateObject(ctx context.Context, req *CreateObjectRequest) (*ObjectActionResponse, error) {
	return &ObjectActionResponse{}, nil
}

// RouterServer Listens for bytes and creates WriteTickets which are
// sent to the putterRequests channel for DataPutter Nodes to write
func RunRouterServer(port int, client WriteNodeClient) error {
	// rpcServer := RegisterRouterServer(rpcServer, &routerServer{})
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
		go routerCreateObject(conn, client)
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

// putterResponseHandler : Handles responses sent from PutterNodes in response to
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
func routerCreateObject(c net.Conn, client WriteNodeClient) error {
	defer c.Close()

	// Specific header prefix for Delete requests which are handled synchronously
	deleteRequestHeader := []byte(DELETE_HEADER)

	// [8B size][1450B data]
	// Limits requests to 16 GB
	// ContentLength must be bigEndian
	contentLenBuf := make([]byte, 8)
	if _, err := c.Read(contentLenBuf); err != nil {
		fmt.Printf("Unable to get the content length for request %s\n", err)
		fmt.Printf("\tReceived: %s\n", string(contentLenBuf))
		return err
	}
	// fmt.Printf("Read contentLen as %s\n", string(contentLenBuf))

	// Delete an object
	// [8B Delete Header][8B ObjectID][16B PSK / Authenticity Token]
	if string(contentLenBuf) == string(deleteRequestHeader) {
		fmt.Printf("Handling delete request\n")
		defer c.Close()
		presharedToken := []byte(DEFAULT_AUTHENTICITY_TOKEN)
		objectIDBuf := make([]byte, 8)
		_, err := c.Read(objectIDBuf)
		if err != nil {
			fmt.Printf("Unable to read delete request objectID: %v\n", err)
			c.Write([]byte("_FAILED_"))
			return err
		}
		fmt.Printf("Handling a delete request for objectID: %s\n", string(objectIDBuf))

		authenticityToken := make([]byte, 16)
		_, err = c.Read(authenticityToken)
		if string(authenticityToken) != string(presharedToken) {
			fmt.Printf("Invalid authenticity token %s\n", string(authenticityToken))
			c.Write([]byte("_FAILED_"))
			return fmt.Errorf("Invalid authenticity token for delete request\n")
		}
		if err := DeleteObject(string(objectIDBuf)); err != nil {
			c.Write([]byte("_FAILED_"))
		} else {
			c.Write(objectIDBuf)
		}
		return nil
	}

	// err := binary.Read(contentLenBuf, binary.BigEndian, &contentLength)
	contentLength := int64(binary.BigEndian.Uint64(contentLenBuf))

	// Grant a new ObjectID for this TCP connection / file
	objectID := NextObjectID()
	// coRequest := &CreateObjectRequest{
	// 	ContentLength: contentLength,
	// 	ObjectID:      objectID,
	// }

	fmt.Printf("Handling router connection for %d-byte Object: %s\n", contentLength, string(objectID))

	fmt.Printf("Opening %d bytes of content from Object %s\n", contentLength, string(objectID))
	var n int
	var err error
	var objBytesCnt = int64(0)

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
			fmt.Printf("Error reading bytes from %d onward: %v\n", objBytesCnt, err)
			return err
		}

		if err == io.EOF {
			fmt.Printf("\tEOF Read %d bytes of object %s\n", objBytesCnt, string(objectID))
			break
		}

		// Create a write request for a WriteNode
		writeRequest := NodeWriteRequest{
			ObjectId:  string(objectID),
			TicketId:  string(ticketID),
			ByteStart: objBytesCnt,
			ByteEnd:   objBytesCnt + int64(n),
			ByteCount: int64(n),
			Data:      dataStream,
		}

		// Create a new object
		if err := CreateObject(writeRequest.ObjectId, writeRequest.TicketId); err != nil {
			fmt.Printf("Unable to create Object %s: %v\n", writeRequest.ObjectId, err)
			return err
		}

		// Set the object status to Writing
		err = SetObjectStatus(writeRequest.ObjectId, ObjectStatus[ObjectWriting])
		if err != nil {
			fmt.Printf("Unable to put object in Writing status: %v\n", err)
			return err
		}

		// Counter of Tickets assigned to the Object
		_, err := TouchTicketCounter(writeRequest.ObjectId)

		// Write the data to some node
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		response, err := client.Write(ctx, &writeRequest)
		defer cancel()
		fmt.Printf("TicketWriteResponse for %s of %s: %d\n", response.TicketId, response.ObjectId, response.Status)
		if err != nil {
			fmt.Printf("Error writing ticket %s of %s to NodeWriter: %v\n",
				writeRequest.TicketId,
				writeRequest.ObjectId,
				err)

		}
		if response.Status != 0 {
			fmt.Printf("Error writing ticket %s of %s, got status %d\n",
				writeRequest.TicketId,
				writeRequest.ObjectId,
				response.Status)
		}

		// Create the ticket in the datastore on the response
		err = CreateTicket(response.TicketId, response.ObjectId, response.NodeId, response.ByteStart, response.ByteEnd, response.ByteCount)
		if err != nil {
			fmt.Printf("Unable to save ticket to datastore: %v\n", err)
			return err
		}
		objBytesCnt += int64(n)
		fmt.Printf("[%d/%d] Read %d of %d bytes\n", objBytesCnt, contentLength, n, contentLength)
		_, err = TouchWriteCounter(response.ObjectId)
		if err != nil {
			fmt.Printf("Unable to update write counter of object %s: %v\n", response.ObjectId, err)
			return err
		}
		if objBytesCnt == contentLength {
			fmt.Printf("\tRead all %d bytes of %d for Object %s\n",
				objBytesCnt,
				contentLength,
				writeRequest.ObjectId,
			)
			break
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
	SetObjectByteSize(string(objectID), objBytesCnt)
	fmt.Printf("Persisted all %d bytes of Object %s\n", objBytesCnt, string(objectID))

	// Send the created objectID to the client
	n, err = c.Write(objectID)
	if err != nil {
		fmt.Printf("Failed to notify client [%d]objectID %s was committed successfully\n", n, objectID)
		return err
	}
	fmt.Printf("OK %d byte of [%d]objectID %s write committed\n", objBytesCnt, n, objectID)

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
