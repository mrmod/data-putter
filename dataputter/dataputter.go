// Package dataputter handles requests from remote systems to store bytes,
// which are associated with an ObjectID, to disk.
//
//     # Practical Operation
//
//     Client -> APIPutRequest{bytes} -> Server
//     Server -> putBytes(PutRequest.new(APIPutRequest)) -> PutResponse
package dataputter

import "os"

// PutRequest Request with data to put somewhere
type PutRequest struct {
	ObjectID           string
	ByteStart, ByteEnd int
	Bytes              []byte
}

// APIPutRequest Request from remote system to put data somewhere
type APIPutRequest struct {
	ObjectID, Checksum string
	Bytes              []byte
}

// PutResponse Response given to remote systems when a file is
// created for a APIPutRequest
type PutResponse struct {
	ObjectID, PutID    string
	ByteStart, ByteEnd int
	NodeID             string
	Success            bool
}

// putBytes: Always write bytes to filename, creating as needed
// Does not create paths
func putBytes(filename string, bytes []byte) error {
	f, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE, 0644)
	defer f.Close()
	if err != nil {
		return err
	}
	_, err = f.Write(bytes)
	if err != nil {
		return err
	}
	return nil
}

// Write Commit an APIPutRequest to disk
// * Accepts a APIPutRequest.
// * Writes Bytes from Request to Disk
func Write(r *APIPutRequest) PutResponse {
	err := putBytes(r.ObjectID, r.Bytes)
	if err != nil {
		return PutResponse{
			Success:  false,
			ObjectID: r.ObjectID,
		}
	}
	return PutResponse{}
}
