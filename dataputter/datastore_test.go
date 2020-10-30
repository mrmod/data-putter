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
