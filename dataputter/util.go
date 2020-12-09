package dataputter

import (
	"log"
	"os"
	"strings"
)

var (
	// dataRoot: Basepath for installing objects/bytes
	dataRoot = "data"
)

// ObjectPathComponents Provides a list of path components for an object
// as chunks of two characters
func ObjectPathComponents(objectID string) []string {
	return strings.Split(objectID, "")
}

// ObjectPathString Provide the object path string
func ObjectPathString(objectID string) string {
	return strings.Join(
		append([]string{dataRoot}, ObjectPathComponents(objectID)...),
		string(os.PathSeparator),
	)

}

// CreateObjectPath Creates the directory structure to store bytes
// of data in the tail'th node of the path components
func CreateObjectPath(objectID string) error {
	filepath := ObjectPathString(objectID)
	log.Printf("CreateObjectPath: %s :%#v:\n", filepath, filepath)
	if strings.Contains(filepath, string(os.PathSeparator)) {
		return os.MkdirAll(filepath, 0755)
	}
	return os.Mkdir(filepath, 0755)
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
