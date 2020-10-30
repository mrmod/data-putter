package dataputter

import "strconv"

// TicketGenerator Generates tickets from "0" to "Z" as byte string
func TicketGenerator(lastTicket []byte) []byte {
	// Base16 to Base10 as int64
	n, _ := strconv.ParseInt(string(lastTicket), 16, 64)

	nextN := n + int64(1)
	// Create a hex representation
	nextTicket := []byte(strconv.FormatInt(nextN, 16))

	for len(nextTicket) < 8 {
		nextTicket = append([]byte("0"), nextTicket...)
	}
	return nextTicket
}
