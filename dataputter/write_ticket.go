package dataputter

import (
	"fmt"

	"log"
)

// WriteTicket Contains a TicketID and symetric key Checksum and authenticity hash
type WriteTicket struct {
	// TicketID: Opaque and stringable
	TicketID []byte
	// Checksum: Authenticity
	Checksum []byte
	// Data: Opaque
	Data []byte
}

type DeleteTicketConfirmation struct {
	// TicketID: Opaque
	TicketID string
	// ObjectID: Opaque
	ObjectID string
	// NodeId
	NodeID string
	// TicketIndex: Index of ticket in the total count
	TicketIndex int64
	// Success: True if the ticket was deleted
	Success bool
}

// ObjectWriteTicket Contains a WriteTicket for a specific object
type ObjectWriteTicket struct {
	// ObjectID : Opaque unique string
	ObjectID string
	// Positions of this Write ticket
	ByteStart, ByteEnd, ByteCount int64
	WriteTicket
}

// DataAllocation : Track where bytes have been written to
type DataAllocation struct {
	ObjectWriteTicket
	PutterNodeID string
}

// Bytes of the WriteTicket.TicketID
func (owt ObjectWriteTicket) GetTicketID() []byte {
	return owt.WriteTicket.GetTicketID()
}
func (owt ObjectWriteTicket) String() string {
	return fmt.Sprintf("Object: %s\nWriteTicket: %s", owt.ObjectID, owt.WriteTicket)
}

// Bytes of the WriteTicket.TicketID bytes
func (wt WriteTicket) GetTicketID() []byte {
	return wt.TicketID
}

func (wt WriteTicket) String() string {
	return fmt.Sprintf("TicketID: %s\n\tChecksum: %s\n\tData:\n%v\n",
		string(wt.TicketID),
		string(wt.Checksum),
		string(wt.Data))
}

// Write The data to a AB/CD/EF/obj file
func (wt WriteTicket) Write() error {
	err := CreateObjectPath(string(wt.TicketID))
	if err != nil {
		log.Printf("Unable to write ticket %s: %v\n", string(wt.TicketID), err)
		return err
	}

	filename := ObjectPathString(string(wt.TicketID)) + "/obj"
	return putBytes(filename, wt.Data)
}

// NewWriteTicket Creates a new write ticket with first 8 bytes of ticketID,
// first 8 bytes of checksum, and all of the data bytes
func NewWriteTicket(ticketID, checksum string, data []byte) WriteTicket {
	return WriteTicket{
		TicketID: []byte(ticketID)[0:8],
		Checksum: []byte(checksum)[0:8],
		Data:     data,
	}
}
