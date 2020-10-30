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

A `TicketID` is a stringable opaque data structure which can be correlated to an `ObjectID` and ultimately a version of a `File`

It counts as `0`, `1`..., `Z`, `00`, `01` and so on.

#### Checksum

The `Checksum` is an authenticity hash of the bytes sent. It's verifiable using a symetrical pre-shared key.

#### Data

An opaque collection of bytes aligned to the default `1500 byte` MTU - `40 Bytes` for TCP overhead.

### Data Putter : Placing Bytes

Bytes are written to the path corresponding to ordered length-2 strings from the `TicketID`. For example, the TicketID `AABBCCDD` will become the filepath `/AA/BB/CC/DD/obj`.

# Running

```
# Startup and shutdown
go run main.go
```

## Running : Router Node

```
go run main.go router
```

Files will be rooted in the path, writing to `./data`, you are in and list on `tcp/5001` for TCP byte-streams which look like `WriteTicket`s.

## Running : Putter Node

```
go run main.go server
```

Files will be rooted in the path you are in and listen on `tcp/5000` for `WriteTicket`s which it replies to with `WriteTicketResponse`.

## Running : Loopback client

Sends data to `127.0.0.1:5000` and requires the Putter Node server to be running.

```
go run main.go loopback
```

# Developing

ETCD ended up being a very poor developer experience due to scattered documentation, poor SEO, and unexpected serialization of inputs.

Redis has taken over.

## Redis Container

Bring up with no auth in development mode listening on `6379`.
```
docker run -it --rm --name putter-redis -p 6379:6379 redis
```

# Building

To build for a release the OS-appropriate `build-docker` script will take care of your needs creating an image named `data-putter`.

You may run the result in router mode with `run-in-docker` for your OS. Keep in mind, Redis may not be on the same network.

See Integration of Redis and DataPutterRouter in Testing to see how networking might be setup.

# Testing

## Integration of Redis and DataPutterRouter

This will allow clients to send data to the DataPutterRouter from the host networks and for the DataPutterRouter to contact a Redis instance.

```
docker network create dataputterNet
docker run -it --rm --name putter-redis -p 6379:6379 --net dataputterNet redis
docker run -it --rm --name putter-router -e REDIS_HOSTPORT=putter-redis:6379 -p 5001:5001 -p 5002:5002 --net dataputterNet data-putter router
```

## Simulation : Accepting Writes

The system can be started as a single node object store with

```
go run main.go router
```

This will start Router and encapsulate a PutterRequest and PutterResponse handler. This operates conceptually as:

```
# Router tcp/5001
while 1 == 1 {
    onConnection {

        createObjectID

        for packet in connectionPackets {

            UpdateObjectTicketCount()
            sendWriteRequest() -> PutterRequestHandler()
            
            if EOF { break }
        }
        waitOnAllWrites
        
        OBJECT WRITTEN OK
    }
}

# PutterRequestHandler(PutterRequest)
WriteData() -> SendPutterResponse()

# PutterResponseHandler(PutterResponse) tcp/5002
MarkTicketComplete()
UpdateObjectTicketWriteCount()
```