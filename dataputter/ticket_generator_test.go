package dataputter

import (
	"testing"
)

func TestTicketGenerator(t *testing.T) {
	var ticketID []byte

	ticketID = TicketGenerator(ticketID)
	if string(ticketID) != "00000001" {
		t.Errorf("Expected '0', got %s\n", string(ticketID))
	}

	ticketID = TicketGenerator(ticketID)
	if string(ticketID) != "00000002" {
		t.Errorf("Expected '00000002', got %s\n", string(ticketID))
	}

	// Tickets Roll over eventually
	ticketID = TicketGenerator([]byte("ffffffff"))
	if string(ticketID) != "100000000" {
		t.Errorf("Expected '100,000,000', got %s\n", string(ticketID))
	}
}
