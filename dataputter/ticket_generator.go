package dataputter

import (
	"fmt"
)

const (
	objectIDCounterKey = "objectIDCounter"
	ticketIDCounterKey = "ticketIDCounter"
)

// Unique ticketID generation
func NextTicketID() []byte {
	v, err := touchCounter(ticketIDCounterKey)
	if err != nil {
		fmt.Printf("Unable to get objectID from %s: %v\n",
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
		fmt.Printf("Unable to get objectID from %s: %v\n",
			objectIDCounterKey, err,
		)
		return []byte{}
	}
	objectID := fmt.Sprintf("%07d", v)

	return []byte(objectID)
}
