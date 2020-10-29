// Datastore Stucture
//
// ETCD stores information about the objectID which have been given out
// and the ticketID associated with each objectID
//
// Ticket Object
//
//     TicketID {objectID: string, nodeID: string, byteStart/End/Count : int64
//
// File Object
//
//     ObjectID {totalBytes: int64}
//
// Tickets have all information about their own allocation
//
// 	/tickets/ticketID/byteStart : 0
// 	/tickets/ticketID/byteEnd   : 2
// 	/tickets/ticketID/byteCount : 2
// 	/tickets/ticketID/node      : NodeID
// 	/tickets/ticketID/object    : ObjectID
// 	/tickets/ticketID/status    : TicketStatus
//
// Objects can resolve tickets as a list of sub keys of themselves
//
// 	/objects/objectID/ticketID : TicketID
// 	/objects/objectID/status   : ObjectStatus
package dataputter

import (
	"context"
	"fmt"
	"strconv"
	"time"

	clientv3 "go.etcd.io/etcd/clientv3"
)

var (
	client    *clientv3.Client
	endpoints = []string{"localhost:2379"}

	// String form of status code int
	TicketStatus = map[int]string{
		TicketNew:   "new",
		TicketSaved: "saved",
		TicketError: "error",
	}

	// String for of object status code int
	ObjectStatus = map[int]string{
		ObjectNew:     "new",
		ObjectSaved:   "saved",
		ObjectError:   "error",
		ObjectWriting: "writing",
	}
)

const (
	// State when Router establishes a ticket in the datastore
	TicketNew = iota
	// State when a Ticket is written to disk
	TicketSaved
	// State whwn a Ticket fails to write
	TicketError
)
const (
	// When there is no such status name
	InvalidTicketStatus = -1

	ObjectNew     = TicketNew
	ObjectSaved   = TicketSaved
	ObjectError   = TicketError
	ObjectWriting = 3
)

func init() {
	config := clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: 5 * time.Second,
	}
	c, err := clientv3.New(config)
	if err != nil {
		fmt.Printf("Unable to establish ETCD connection: %v\n", err)
		return
	}
	fmt.Println("Created ETCD client connection")
	client = c
}

// Get the ticket status code by name
func GetTicketStatusCode(statusName string) int {
	for code, name := range TicketStatus {
		if name == statusName {
			return code
		}
	}
	return InvalidTicketStatus
}

// Value is a string or string-ed []byte. Etcd assures us this is OK
// http://localhost:4001/pkg/go.etcd.io/etcd/clientv3/#KV
func writeString(keyPath, value string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, err := client.KV.Put(ctx, keyPath, value)

	if err != nil {
		fmt.Printf("Error putting key %s with val %s: %v\n", keyPath, value, err)
		return err
	}

	written, err := client.KV.Get(ctx, keyPath)
	if err != nil {
		fmt.Printf("Unable to verify %s\n", keyPath)
		return err
	} else {
		fmt.Printf("Stored %s (%s requested) at keyPath %s\n", written.Kvs[0].Value, value, keyPath)
	}

	return nil
}

// Create a new object reference
func CreateObject(objectID, ticketID string) error {
	var err error
	basePath := "/objects/" + objectID + "/"

	err = writeString(basePath+ticketID, ticketID)
	if err != nil {
		return err
	}

	err = writeString(basePath+"status", TicketStatus[TicketNew])
	return err
}

// Sets a new Object status
func SetObjectStatus(objectID, status string) error {
	return writeString(
		"/objects/"+objectID+"/status",
		status,
	)
}

// Sets a new ticket status
func SetTicketStatus(ticketID, status string) error {
	fmt.Printf("SetTicketStatus of %s to %s\n", ticketID, status)
	return writeString(
		"/tickets/"+ticketID+"/status",
		status,
	)
}

// Create a new ticket
func CreateTicket(ticketID, objectID, nodeID string, byteStart, byteEnd, byteCount int64) error {
	var err error

	basePath := "/tickets/" + ticketID + "/"
	err = writeString(basePath+"ticket", ticketID)
	if err != nil {
		return err
	}
	err = writeString(basePath+"object", objectID)
	if err != nil {
		return err
	}
	err = writeString(basePath+"node", nodeID)
	if err != nil {
		return err
	}

	err = writeString(basePath+"byteStart", strconv.FormatInt(byteStart, 10))
	if err != nil {
		return err
	}
	err = writeString(basePath+"byteEnd", strconv.FormatInt(byteEnd, 10))
	if err != nil {
		return err
	}
	err = writeString(basePath+"byteCount", strconv.FormatInt(byteCount, 10))
	if err != nil {
		return err
	}

	err = writeString(basePath+"status", TicketStatus[TicketNew])
	if err != nil {
		return err
	}

	return err
}
