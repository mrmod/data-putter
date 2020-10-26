package dataputter

import "testing"

func TestTicketGenerator(t *testing.T) {
	var ticketID []byte

	ticketID = TicketGenerator(ticketID)

	if string(ticketID) != "0" {
		t.Errorf("expected '0', len %d is %s", len(ticketID), string(ticketID))
	}

	ticketID = TicketGenerator(ticketID)

	if string(ticketID) != "1" {
		t.Errorf("expected '1', len %d got %s\n", len(ticketID), string(ticketID))
	}

	ticketID = TicketGenerator([]byte("Z"))
	if string(ticketID) != "00" {
		t.Errorf("expected '00', len %d got %s\n", len(ticketID), string(ticketID))
	}

	ticketID = TicketGenerator([]byte("00"))
	if string(ticketID) != "01" {
		t.Errorf("expected '01', len %d got %s\n", len(ticketID), string(ticketID))
	}
	ticketID = TicketGenerator([]byte("ZZ"))
	if string(ticketID) != "000" {
		t.Errorf("expected '000', len %d got %s\n", len(ticketID), string(ticketID))
	}
	ticketID = TicketGenerator([]byte("010"))
	if string(ticketID) != "011" {
		t.Errorf("expected '011', len %d got %s\n", len(ticketID), string(ticketID))
	}

	ticketID = TicketGenerator([]byte("2ZZ"))
	if string(ticketID) != "3ZZ" {
		t.Errorf("expected '3ZZ', len %d got %s\n", len(ticketID), string(ticketID))
	}
}
