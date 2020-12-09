package dataputter

import (
	"fmt"
	"log"
)

const (
	objectIDCounterKey = "objectIDCounter"
	ticketIDCounterKey = "ticketIDCounter"
)

// Unique ticketID generation
func NextTicketID() []byte {
	v, err := touchCounter(ticketIDCounterKey)
	if err != nil {
		log.Printf("Unable to get objectID from %s: %v\n",
			objectIDCounterKey, err,
		)
		return []byte{}
	}
	ticketID := fmt.Sprintf("%08d", v)

	return []byte(ticketID)
}

// Unique objectID generation
func NextObjectID() []byte {
	v, err := touchCounter(objectIDCounterKey)
	if err != nil {
		log.Printf("Unable to get objectID from %s: %v\n",
			objectIDCounterKey, err,
		)
		return []byte{}
	}
	objectID := fmt.Sprintf("%08d", v)

	return []byte(objectID)
}
