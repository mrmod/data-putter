# Simple Object Store

Stores bytes on disks.

```
Client -> [8B TicketID | 8B Checksum | 1458B Data] -> Server

Server -> WriteDataToDisk() -> [8B TicketID | 1B Status] -> Client
```

## Overview

Simple Object Store consists of a DataPutter, DataReceiver, and Datastore.

* A File is sent to **DataReceiver** who turns it into 1458-byte chunks.
* Each chunk is sent to a **DataPutter** to store
* When all chunks are stored, the file has been "Received"

### Data Putter : Writing Bytes

DataPutter receives requests of the structure:

```
---------------- ---------------- ----~----
| 8B            | 8B             | 1458B   |
| TicketID      |Checksum        |Data     |
---------------- ---------------- ---------
```

#### TicketID

A `TicketID` is a stringable MOD 2 opaque data structure which can be correlated to an `ObjectID` and ultimately a version of a `File`

#### Checksum

The `Checksum` is an authenticity hash of the bytes sent. It's verifiable using a symetrical pre-shared key.

#### Data

An opaque collection of bytes aligned to the default `1500 byte` MTU - `40 Bytes` for TCP overhead.

### Data Putter : Placing Bytes

Bytes are written to the path corresponding to ordered length-2 strings from the `TicketID`. For example, the TicketID `AABBCCDD` will become the filepath `/AA/BB/CC/DD/obj`.

# Running

```
go run main.go
```

Files will be rooted in the path you are in and listen on `tcp/5000` for `WriteTicket`s which it replies to with `WriteTicketResponse`.