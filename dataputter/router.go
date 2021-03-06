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
	"log"
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

type routerServer struct {
	UnimplementedRouterServer
	Config RouterConfig
}

func (s *routerServer) CreateObject(ctx context.Context, req *CreateObjectRequest) (*ObjectActionResponse, error) {
	return &ObjectActionResponse{}, nil
}

// RouterServer Listens for bytes and creates WriteTickets which are
// sent to the putterRequests channel for DataPutter Nodes to write
func RunRouterServer(config RouterConfig) error {
	port := config.Port

	// rpcServer := RegisterRouterServer(rpcServer, &RouterServer{})

	s, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return err
	}
	log.Printf("PutterRouter running on port %d\n", port)
	for {
		conn, err := s.Accept()
		if err != nil {
			log.Printf("Error in connection: %v\n", err)
			continue
		}
		// Handle a request to store a file/bunch-of-bytes somewhere
		go DoCreateObject(conn, config)
	}
}

// putterRequestHandler : Handles a single TCP connection creating an
// ObjectID for the file and then WriteTickets for each byte region.
// When it has written all the bytes sent, it will reply with the 8 Byte
// ObjectID it has assigned.
func DoCreateObject(c net.Conn, config RouterConfig) error {
	defer c.Close()

	// Specific header prefix for Delete requests which are handled synchronously
	deleteRequestHeader := []byte(DELETE_HEADER)

	// [8B size][1450B data]
	// Limits requests to 16 GB
	// ContentLength must be bigEndian
	contentLenBuf := make([]byte, 8)
	if _, err := c.Read(contentLenBuf); err != nil {
		log.Printf("Unable to get the content length for request %s\n", err)
		log.Printf("\tReceived: %s\n", string(contentLenBuf))
		return err
	}
	// log.Printf("Read contentLen as %s\n", string(contentLenBuf))

	// Delete an object
	// [8B Delete Header][8B ObjectID][16B PSK / Authenticity Token]
	if string(contentLenBuf) == string(deleteRequestHeader) {
		log.Printf("Handling delete request\n")
		defer c.Close()
		objectIDBuf := make([]byte, 8)
		_, err := c.Read(objectIDBuf)
		if err != nil {
			log.Printf("Unable to read delete request objectID: %v\n", err)
			c.Write([]byte("_FAILED_"))
			return err
		}

		log.Printf("Handling a delete request for objectID: %s\n", string(objectIDBuf))

		// DeleteTicketHandler
		tickets, err := DeleteObject(
			string(objectIDBuf),
		)
		if err != nil {
			log.Printf("Failed to delete %s: %v\n", string(objectIDBuf), err)
			c.Write([]byte("_FAILED_"))
			return err
		}
		c.Write(objectIDBuf)
		for _, ticket := range tickets {
			log.Printf("Deleted %s\n", ticket)
		}
		return nil
	}

	contentLength := int64(binary.BigEndian.Uint64(contentLenBuf))

	// Grant a new ObjectID for this TCP connection / file
	objectID := NextObjectID()

	log.Printf("Handling router connection for %d-byte Object: %s\n", contentLength, string(objectID))

	log.Printf("Opening %d bytes of content from Object %s\n", contentLength, string(objectID))
	var n int
	var err error
	var objBytesCnt = int64(0)

	var writeInProgress sync.WaitGroup
	writeInProgress.Add(1)

	// Consumers waiting on Object write to complete
	writeWaiters := make(chan CounterEvent, 2)

	// Write regions of bytes for this object
	nodeIndex := 0
	for {
		log.Printf("-- -- --\n")
		dataStream := make([]byte, 1450)
		ticketID := NextTicketID()
		log.Printf("Trying to read bytes from Object %s stream\n", string(objectID))
		n, err = c.Read(dataStream)
		log.Printf("\tRead %d bytes from Object %s stream\n", n, string(objectID))
		if err != nil && err != io.EOF {
			log.Printf("Error reading bytes from %d onward: %v\n", objBytesCnt, err)
			return err
		}

		if err == io.EOF {
			log.Printf("\tEOF Read %d bytes of object %s\n", objBytesCnt, string(objectID))
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
			log.Printf("Unable to create Object %s: %v\n", writeRequest.ObjectId, err)
			return err
		}

		// Set the object status to Writing
		err = SetObjectStatus(writeRequest.ObjectId, ObjectStatus[ObjectWriting])
		if err != nil {
			log.Printf("Unable to put object in Writing status: %v\n", err)
			return err
		}

		// Counter of Tickets assigned to the Object
		_, err := TouchTicketCounter(writeRequest.ObjectId)

		// TODO: Should use service lookup to find nodes
		// during each segment
		nodeConfig := config.Nodes[nodeIndex]
		nodeAddress := fmt.Sprintf("%s:%d", nodeConfig.Host, nodeConfig.Port)
		fmt.Println("Creating nodeClient")
		nodeClient, err := NewClient(nodeAddress)
		fmt.Println("Created nodeClient")
		if err != nil {
			log.Printf("Unable to create NodeClient: %v\n", err)
			return err
		}
		defer nodeClient.Close()

		// Write the data to some node
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		response, err := nodeClient.Write(ctx, &writeRequest)
		defer cancel()
		log.Printf("TicketWriteResponse for %s of %s: %d\n", response.TicketId, response.ObjectId, response.Status)
		if err != nil {
			log.Printf("Error writing ticket %s of %s to NodeWriter: %v\n",
				writeRequest.TicketId,
				writeRequest.ObjectId,
				err)

		}
		// Use the next node for the next ticket
		if nodeIndex < len(config.Nodes)-1 {
			nodeIndex++
		} else {
			nodeIndex = 0
		}

		if response.Status != 0 {
			log.Printf("Error writing ticket %s of %s, got status %d\n",
				writeRequest.TicketId,
				writeRequest.ObjectId,
				response.Status)
		}

		// Create the ticket in the datastore on the response
		err = CreateTicket(response.TicketId, response.ObjectId, response.NodeId, response.ByteStart, response.ByteEnd, response.ByteCount)
		if err != nil {
			log.Printf("Unable to save ticket to datastore: %v\n", err)
			return err
		}
		objBytesCnt += int64(n)
		log.Printf("[%d/%d] Read %d of %d bytes\n", objBytesCnt, contentLength, n, contentLength)
		_, err = TouchWriteCounter(response.ObjectId)
		if err != nil {
			log.Printf("Unable to update write counter of object %s: %v\n", response.ObjectId, err)
			return err
		}
		if objBytesCnt == contentLength {
			log.Printf("\tRead all %d bytes of %d for Object %s\n",
				objBytesCnt,
				contentLength,
				writeRequest.ObjectId,
			)
			break
		}
	}
	// At this point we have access to the length of the object in bytes
	// Wait for the file to be written
	log.Printf("Waiting for Object %s to be written\n", string(objectID))
	go spinWhileObjectWriting(string(objectID), writeWaiters, &writeInProgress)
	go WatchCounter("/objects/"+string(objectID)+"/writeCounter", writeWaiters)
	log.Printf("Waiting for WatchCounter to release inProgress for %s\n", string(objectID))
	writeInProgress.Wait()
	// close(writeWaiters)
	SetObjectByteSize(string(objectID), objBytesCnt)
	log.Printf("Persisted all %d bytes of Object %s\n", objBytesCnt, string(objectID))

	// Send the created objectID to the client
	n, err = c.Write(objectID)
	if err != nil {
		log.Printf("Failed to notify client [%d]objectID %s was committed successfully\n", n, objectID)
		return err
	}
	log.Printf("OK %d byte of [%d]objectID %s write committed\n", objBytesCnt, n, objectID)

	return c.Close()
}

func spinWhileObjectWriting(objectID string, countEvents chan CounterEvent, wg *sync.WaitGroup) {

	log.Printf("Wait on object write to complete")
	for countEvent := range countEvents {
		writeCounter, wcErr := GetWriteCounterValue(objectID)
		ticketCounter, tcErr := GetTicketCounterValue(objectID)

		if wcErr == nil && tcErr == nil {
			log.Printf("Comparing T: %d with W %d vs CE %s :: %d\n",
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
			log.Printf("Counter event errors; %v and %v\n", wcErr, tcErr)
		}
	}
}
