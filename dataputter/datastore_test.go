package dataputter

import "testing"

func TestSetTicketStatus(t *testing.T) {
	var err error
	err = SetTicketStatus("ticketID", TicketStatus[TicketNew])
	if err != nil {
		t.Errorf("Expected no error, got %v\n", err)
	}

	status, err := GetTicketStatus("ticketID")
	if err != nil {
		t.Errorf("Expected no errors, got %v\n", err)
	}
	if status != "new" {
		t.Errorf("Expected 'new', got %s\n", status)
	}
}

func TestSetTicketStatusInSequence(t *testing.T) {

	tests := []string{"new", "saved", "error", "saved"}
	for _, status := range tests {
		err := SetTicketStatus("ticketID", status)
		if err != nil {
			t.Errorf("Expected to set initial status of %s but failed\n", status)
		}

		storedStatus, err := GetTicketStatus("ticketID")
		if err != nil {
			t.Errorf("Expected to get status for %s but failed\n", status)
		}
		if status != storedStatus {
			t.Errorf("Expected ticket status %s, got %s\n", storedStatus, status)
		}
	}
}

func TestGetObjectTickets(t *testing.T) {
	defer DeleteTicket("TEST_OBJECT_ID", "TEST_TICKET_ID")
	err := CreateTicket("TEST_TICKET_ID", "TEST_OBJECT_ID", "TEST_NODE_ID", 0, 10, 10)
	if err != nil {
		t.Errorf("Expected to write one ticket, got %v\n", err)
	}

	tickets, err := GetObjectTickets("TEST_OBJECT_ID")
	if err != nil {
		t.Errorf("Expected tickets for 'TEST_OBJECT_ID', got %v\n", err)
	}
	if len(tickets) != 1 {
		t.Errorf("Expected 1 ticket, got %d\n", len(tickets))
	}
}
