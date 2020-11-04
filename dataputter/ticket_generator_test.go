package dataputter

import (
	"testing"
)

func TestNextTicketID(t *testing.T) {
	deleteKeyPath(ticketIDCounterKey)

	for i := 0; i < 10; i++ {
		id := NextTicketID()
		if len(id) != 8 {
			t.Errorf("Expected 8 bytes, got %d\n", len(id))
		}
	}
}
