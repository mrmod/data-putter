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
	"fmt"
	"os"
	"strconv"
	"time"

	redis "github.com/mediocregopher/radix/v3"
)

var (
	client    *redis.Pool
	endpoints = []string{"localhost:6379"}

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
	ObjectNew = iota
	ObjectSaved
	ObjectError
	ObjectWriting
)

const (
	// When there is no such status name
	InvalidTicketStatus = -1
)

func init() {
	redisHostport := os.Getenv("REDIS_HOSTPORT")
	if len(redisHostport) > 0 {
		endpoints[0] = redisHostport
	}
	c, err := redis.NewPool("tcp", endpoints[0], 10)
	if err != nil {
		fmt.Printf("Unable to establish Redis connection: %v\n", err)
		return
	}
	fmt.Println("Created Redis connection")
	client = c
}

// Value is a string or string-ed []byte. Etcd assures us this is OK
// http://localhost:4001/pkg/go.etcd.io/etcd/clientv3/#KV
func writeString(keyPath, value string) error {
	return client.Do(
		redis.Cmd(nil, "SET", keyPath, value),
	)
}

func getKey(keyPath string) (string, error) {
	fmt.Printf("GetKey: %s ::%#v::\n", keyPath, keyPath)
	var value string
	err := client.Do(
		redis.Cmd(&value, "GET", keyPath),
	)
	return value, err
}

func getCounter(keyPath string) (int64, error) {
	var value int64
	err := client.Do(
		redis.Cmd(&value, "GET", keyPath),
	)
	return value, err
}

func initCounter(keyPath string) error {
	return client.Do(redis.Cmd(nil, "SET", keyPath, "0"))
}

func touchCounter(keyPath string) (int64, error) {
	var value int64

	err := client.Do(
		redis.Cmd(&value, "INCR", keyPath),
	)
	return value, err
}

func deleteKeyPath(keyPath string) error {
	return client.Do(redis.Cmd(nil, "DEL", keyPath))
}

// Touches a ticket counter. Each touch updates the Version of the key
// https://groups.google.com/g/etcd-dev/c/8xVPAkUfWdM?pli=1
func TouchTicketCounter(objectID string) (string, error) {
	// Each put will generate a new version of the key in etcd
	// this is a good point to set an abstraction
	keyPath := "/objects/" + objectID + "/ticketCounter"
	_, err := touchCounter(keyPath)
	return keyPath, err
}

// Touches a write counter. Each touch updates the Version of the key
// https://groups.google.com/g/etcd-dev/c/8xVPAkUfWdM?pli=1
func TouchWriteCounter(objectID string) (string, error) {
	keyPath := "/objects/" + objectID + "/writeCounter"
	_, err := touchCounter(keyPath)
	return keyPath, err
}

// Reduces the ticket counter by 1
func ReduceTicketCounter(objectID string) (int64, error) {
	var value int64
	keyPath := "/objects/" + objectID + "/ticketCounter"
	return value, client.Do(
		redis.Cmd(&value, "DECR", keyPath),
	)
}

// Emitted to observers of a WatchCounter
type CounterEvent struct {
	// KeyPath of the counter
	KeyPath string
	// Value of the counter
	Value int64
}

// Watches a key for Version updates
func WatchCounter(keyPath string, observers chan CounterEvent) error {
	fmt.Printf("WatchCounter starting for %s\n", keyPath)

	if v, err := getCounter(keyPath); err != nil {
		fmt.Printf("Unable to get counter %s\n", keyPath)
		return err
	} else {
		observers <- CounterEvent{keyPath, v}
	}

	// 200 Hz
	for range time.Tick(time.Millisecond * 5) {
		v, err := getCounter(keyPath)
		if err == nil {
			observers <- CounterEvent{
				keyPath,
				v,
			}
		}
	}

	return nil
}

func GetTicketCounterValue(objectID string) (int64, error) {
	return getCounter("/objects/" + objectID + "/ticketCounter")
}

func GetWriteCounterValue(objectID string) (int64, error) {
	return getCounter("/objects/" + objectID + "/writeCounter")
}

func GetTicketObject(ticketID string) (string, error) {
	return getKey("/tickets/" + ticketID + "/object")
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

// Create a new object reference and ticket reference
// Creates: /objects/$objectID/tickets/$ticketID = ticketID
// Adds to set: objects { objectID }
// Sets object status: /objects/$objectID/status NEW
func CreateObject(objectID, ticketID string) error {
	var err error
	basePath := "/objects/" + objectID + "/"

	err = writeString(basePath+"tickets/"+ticketID, ticketID)
	if err != nil {
		return err
	}

	err = client.Do(redis.Cmd(nil, "SADD", "objects", objectID))
	if err != nil {
		fmt.Printf("Unable to add %s to set of objects: %v", objectID, err)
	}

	err = writeString(basePath+"status", TicketStatus[TicketNew])
	return err
}

// Set the size of an object
func SetObjectByteSize(objectID string, sizeInBytes int64) error {
	return writeString(
		"/objects/"+objectID+"/size",
		fmt.Sprintf("%d", sizeInBytes),
	)
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
	keyPath := "/tickets/" + ticketID + "/status"

	return writeString(
		keyPath,
		status,
	)
}

// Get a list of tickets for in the inclusive range from offset to minByt + 512KB
func GetTicketsFromOffset(objectID string, offset int64) (tickets []string, err error) {
	fmt.Printf("GetTicketFromOffset %d %s\n", offset, objectID)
	err = client.Do(
		redis.Cmd(
			&tickets,
			"ZRANGEBYSCORE",
			"objectBytes/"+objectID,
			strconv.FormatInt(offset, 10),
			strconv.FormatInt(offset+int64(1024*512), 10),
		),
	)

	if err != nil {
		return
	}

	return
}

// Create a new ticket in the datastore
// Creates: /tickets/$ticketID/ticket = ticketID
// Creates: /tickets/$TICKET_ID/object = OBJECT_ID
// Creates: /tickets/$TICKET_ID/node = NODE_ID
// Sets /tickets/$TICKET_ID/byteStart = byteStart
// Sets /tickets/$TICKET_ID/byteEnd = byteEnd
// Sets /tickets/$TICKET_ID/byteCount = byteCount
// Adds byteStart position to set of objectBytes: objectBytes/$objectID { byteStart }
// Adds ticket to set of Object tickets: objectTickets/$objectID { ticketID }
// Adds node to set of nodes containing tickets: objectNodes/$objectID { nodeID }
func CreateTicket(ticketID, objectID, nodeID string, byteStart, byteEnd, byteCount int64) error {
	var err error
	fmt.Printf("[%d:%d] CreateTicket %s for object %s\n", byteStart, byteEnd, ticketID, objectID)
	basePath := "/tickets/" + ticketID + "/"

	// /tickets/$TICKET_ID/ticket = TICKET_ID
	err = writeString(basePath+"ticket", ticketID)
	if err != nil {
		return err
	}
	// /tickets/$TICKET_ID/object = OBJECT_ID
	err = writeString(basePath+"object", objectID)
	if err != nil {
		return err
	}
	// /tickets/$TICKET_ID/node = NODE_ID
	err = writeString(basePath+"node", nodeID)
	if err != nil {
		return err
	}
	// /tickets/$TICKET_ID/byteStart = 0
	err = writeString(basePath+"byteStart", strconv.FormatInt(byteStart, 10))
	if err != nil {
		return err
	}
	// /tickets/$TICKET_ID/byteEnd = 1450
	err = writeString(basePath+"byteEnd", strconv.FormatInt(byteEnd, 10))
	if err != nil {
		return err
	}
	// /tickets/$TICKET_ID/byteCount = 1450
	err = writeString(basePath+"byteCount", strconv.FormatInt(byteCount, 10))
	if err != nil {
		return err
	}

	// Keep tickets sorted for an object by storing the start byte as the score of a ticket
	err = client.Do(
		redis.Cmd(nil, "ZADD", "objectBytes/"+objectID, strconv.FormatInt(byteStart, 10), ticketID),
	)
	if err != nil {
		return err
	}

	// Track which tickets an object has
	err = client.Do(redis.Cmd(nil, "SADD", "objectTickets/"+objectID, ticketID))
	if err != nil {
		fmt.Printf("Unable to add %s to set of objects: %v", objectID, err)
	}

	// Track which nodes have tickets for an object
	err = client.Do(redis.Cmd(nil, "SADD", "objectNodes/"+objectID, nodeID))
	if err != nil {
		fmt.Printf("Unable to add %s to set of objects: %v", objectID, err)
	}

	// SetTicketStatus(ticketID, TicketStatus[TicketNew])

	return err
}

func GetTicketStatus(ticketID string) (string, error) {
	return getKey("/tickets/" + ticketID + "/status")
}

func GetTicketNode(ticketID string) (string, error) {
	return getKey("/tickets/" + ticketID + "/node")
}

func GetTicketSize(ticketID string) (int64, error) {
	v, err := getKey("/tickets/" + ticketID + "/byteCount")

	if err != nil {
		return int64(0), err
	}
	return strconv.ParseInt(v, 10, 64)
}

func GetObjectTickets(objectID string) ([]string, error) {
	tickets := []string{}

	err := client.Do(
		redis.Cmd(&tickets, "smembers", "objectTickets/"+objectID),
	)

	return tickets, err
}

func DeleteTicket(objectID, ticketID string) error {
	status, err := GetTicketStatus(ticketID)
	if err != nil {
		return err
	}
	if status != "saved" {
		return fmt.Errorf("Denying access to ticket %s in state %s\n", ticketID, status)
	}
	// Remove ticket from set of object tickets
	keyPath := "objectTickets/" + objectID

	if err := client.Do(
		redis.Cmd(nil, "srem", keyPath, ticketID),
	); err != nil {
		return err
	}

	keyPaths := []string{
		"/tickets/" + ticketID + "/byteCount",
		"/tickets/" + ticketID + "/node",
		"/tickets/" + ticketID + "/status",
		"/tickets/" + ticketID + "/ticket",
		"/tickets/" + ticketID + "/object",
		"/tickets/" + ticketID + "/byteStart",
		"/tickets/" + ticketID + "/byteEnd",
		"/tickets/" + ticketID + "/byteCount",
	}
	for _, path := range keyPaths {
		if err := client.Do(redis.Cmd(nil, "del", path)); err != nil {
			fmt.Printf("Failed to delete %s: %v\n", path, err)
			return err
		}
	}
	return err
}

func DeleteObjectReference(objectID string) error {
	fmt.Printf("DeleteObjectReference %s\n", objectID)
	keyPaths := []string{
		// Delete set of tickets associated with the object
		"objectTickets/" + objectID,
		// Delete set of nodes the object was written to
		"objectNodes/" + objectID,
		// Delete min heap of ticket ids
		"objectBytes/" + objectID,
		"/objects/" + objectID + "/size",
		"/objects/" + objectID + "/status",
		"/objects/" + objectID + "/writeCounter",
		"/objects/" + objectID + "/ticketCounter",
	}

	for _, keyPath := range keyPaths {
		if err := client.Do(redis.Cmd(nil, "del", keyPath)); err != nil {
			fmt.Printf("Failed to delete %s: %v\n", keyPath, err)
			return err
		}
	}

	// Finally, delete the object from the set of objects
	return client.Do(
		redis.Cmd(nil, "SREM", "objects", objectID),
	)
}

func GetObjectSize(objectID string) (int64, error) {
	v, err := getKey("/objects/" + objectID + "/size")

	if err != nil {
		return int64(0), err
	}
	return strconv.ParseInt(v, 10, 64)
}
