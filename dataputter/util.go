// Package dataputter Utilities
package dataputter

import (
	"os"
	"strings"
)

// ObjectPathComponents Provides a list of path components for an object
// as chunks of two characters
func ObjectPathComponents(objectID string) []string {
	s := make([]string, len(objectID)/2)
	for i := 0; i < len(objectID); i += 2 {
		s[i/2] = objectID[i : i+2]
	}
	return s
}

// ObjectPathString Provide the object path string
func ObjectPathString(objectID string) string {
	return strings.Join(
		ObjectPathComponents(objectID),
		string(os.PathSeparator),
	)
}

// CreateObjectPath Creates the directory structure to store bytes
// of data in the tail'th node of the path components
func CreateObjectPath(objectID string) error {
	filepath := ObjectPathString(objectID)
	return os.MkdirAll(filepath, 0755)
}

// StoreBytes Stores the bytes from a write ticket somewhere
// on a disk
func StoreBytes(ticket WriteTicket) error {
	err := CreateObjectPath(string(ticket.TicketID))
	if err != nil {
		return err
	}
	return ticket.Write()
}
