# Simple Object Store

Stores bytes on disks.

```
Client -> [8B ContentLength | 1450B Data] -> Router

Router -> GRPC -> WriteNode

WriteNode -> WriteDataToDisk() -> GRPC -> Router.NodeClient
```

## Overview

Simple Object Store consists of a WriteNode, Router, and Datastore.

* A File is sent to **Router** who turns it into 1450-byte chunks.
* Each chunk is sent to a **WriteNode** to store
* When all chunks are stored, the file has been "Received"

### Roles and Ports

```
# RunsOn: Router
# Handles requests to store bytes of an object
Router: 5001

# RunsOn: Router
# Handles Read requests
ObjectRequestServer: 5004

# RunsOn: WriteNode
# Handles Read/Write/Delete of bytes/Tickets
WriteNode: 5002
```

## Topologies

### Data Topology

Redis is used for all datastructures

```
# Object is a whole thing of bytes
SET /objects {Object1, Object2}
# Ticket is part of an object
SET objectTickets/$OBJECT_ID {Ticket1, Ticket2}
# Node is a place where bytes can be written
SET objectNodes/$OBJECT_ID {Node1, Node1}
```

Concurrency is managed using the datastructure server as well

```
# Number of tickets created for an object
INT /objects/$OBJECT_ID/ticketCounter 1
# Number of tickets written of an object
INT /objects/$OBJECT_ID/writeCounter 1
```

Object Metadata is stored in the same way

```
# Content type
STRING /objects/$OBJECT_ID/contentType

# Size in bytes
INT /objects/$OBJECT_ID/size 
```

### RPC Topology

## Router

```
TCP -> Router:5001 -> WriteNode.RPC[Write, Delete]
TCP -> Router:5004 -> WriteNode.RPC[Read]
```

## Write Node

```
Router -> WriteNode.Read( NodeReadRequest ) -> NodeResponse
Router -> WriteNode.Write( NodeWriteRequest ) -> NodeResponse
Router -> WriteNode.Delete( NodeDeleteRequest ) -> NodeResponse

```

#### TicketID

A `TicketID` is a stringable opaque data structure which can be correlated to an `ObjectID` and ultimately a version of a `File`

It counts as `0`, `1`..., `Z`, `00`, `01` and so on.

#### Checksum

The `Checksum` is an authenticity hash of the bytes sent. It's verifiable using a symetrical pre-shared key.

#### Data

An opaque collection of bytes aligned to the default `1500 byte` MTU - `40 Bytes` for TCP overhead.

### Data Putter : Placing Bytes

Bytes are written to the path corresponding to ordered length-2 strings from the `TicketID`. For example, the TicketID `AABBCCDD` will become the filepath `/A/A/B/B/C/C/D/D/obj`.

# Running

The whole stack can run locally using the `standAlone` mode

```
go run main.go standAlone

# tcp/5001 - Object Write/Delete
# tcp/5004 - Object Read

# tcp/5002 - Ticket Read/Write/Delete
```

# Developing

## Compiling Proto

```
compile_proto.ps1
```

## Redis Container

Bring up with no auth in development mode listening on `6379`.

```
docker run -it --rm --name putter-redis -p 6379:6379 redis
```
