// Package dataputter The router is responsible for listening for an entire
// file of bytes from an object owner. It reads at least 1458 bytes into memory.
//
// Bytes are ready in 1458-byte chunks until a `nil` is encountered. Each time a
// chunk is ready, it is assigned a ticket and a checksum is done on the bytes.
//
// After the ticket, a WriteTicket and checksum are created, they are sent along with
// their data bytes to a PutterNode so it can write them to disk somewhere.
package dataputter

import (
	"fmt"
	"io"
	"net"
)

// RouterServer Listens for bytes and creates WriteTickets
func RouterServer(port int, ticketIntake chan WriteTicket) error {
	s, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return err
	}
	fmt.Printf("PutterRouter running on port %d\n", port)
	for {
		conn, err := s.Accept()
		if err != nil {
			fmt.Printf("Error in connection: %v\n", err)
			continue
		}
		go streamHandler(conn, ticketIntake)
	}
}

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

func streamHandler(c net.Conn, ticketIntake chan WriteTicket) error {
	defer c.Close()
	fmt.Println("Handling router connection")
	dataStream := make([]byte, 1458)
	var err error
	var n int
	var byteCount = 0
	var ticketID []byte
	for {
		ticketID = TicketGenerator(ticketID)
		n, err = c.Read(dataStream)
		if err != nil && err != io.EOF {
			fmt.Printf("Error reading bytes from %d onward: %v\n", byteCount, err)
			return err
		}
		if n > 0 {
			byteCount += n
			fmt.Printf("Read %d of %d bytes\n", n, byteCount)
			ticketIntake <- WriteTicket{
				TicketID: ticketID,
				Checksum: make([]byte, 8),
				Data:     dataStream,
			}
		}
		if err != nil && err == io.EOF {
			break
		}
	}

	return nil
}
