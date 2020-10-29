package dataputter

import "fmt"

// TicketGenerator Generates tickets from "0" to "Z" as byte string
func TicketGenerator(lastTicket []byte) []byte {
	if len(lastTicket) == 0 {
		return []byte("0")
	}
	// 8 bytes is the limit (check for Z/90)
	if len(lastTicket) == 8 && lastTicket[0] == 90 {
		fmt.Println("Warning: End of tickets")
		return lastTicket
	}
	newTicket := make([]byte, len(lastTicket))
	copy(newTicket, lastTicket)

	// A(65) - Z(90)
	// 0(48) - 9(57)
	// a(97) - z(122)
	for i := len(lastTicket) - 1; i >= 0; i-- {
		d := lastTicket[i]
		// 0 - 9
		if d >= 48 && d < 57 {
			newTicket[i] = d + 1
			break
			// A - Z
		} else if d == 57 || (d >= 65 && d < 90) {
			if d == 57 {
				// Move 9 to A
				newTicket[i] = 65
				break
			} else {
				newTicket[i] = d + 1
				break
			}
			// a - z
			// Disabled for windows
			// } else if d == 90 || (d >= 97 && d < 122) {
			// 	if d == 90 {
			// 		// Move Z to a
			// 		newTicket[i] = 97
			// 	} else {
			// 		newTicket[i] = d + 1
			// 	}
			// initialize to 0
		} else if d == 0 {
			return append([]byte{48}, newTicket...)

		} else {
			// Need to add more bytes for short strings
			// If this is the 0th byte, no more allocations can be done)
			if i == 0 {
				for i := 0; i < len(newTicket); i++ {
					newTicket[i] = byte(48)
				}
				return append([]byte{48}, newTicket...)
			}
		}

	}

	return newTicket
}
