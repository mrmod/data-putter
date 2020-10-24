package dataputter

import "fmt"

// WriteTicket Contains a TicketID and symetric key Checksum and authenticity hash
type WriteTicket struct {
	// TicketID: Opaque and stringable
	TicketID []byte
	// Checksum: Authenticity
	Checksum []byte
	// Data: Opaque
	Data []byte
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
		return err
	}

	filename := ObjectPathString(string(wt.TicketID)) + "/obj"
	return putBytes(filename, wt.Data)
}
