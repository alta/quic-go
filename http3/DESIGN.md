Here’s a rough design for where I landed with this. In the spirit of “smaller interfaces are better”:

### `http3.Server`

To enable HTTP/3 extensions, I propose adding a new field to `http3.Server`: `Requester`. If set, it allows callers to intercept an accepted `quic.EarlySession` and provide an `http3.Requester`. It would default to `http3.Accept`, a new exported func that would tie together the QUIC session with a QPACK decoder and some other state (see below).

```go
	// If set, the server will call Requester for each accepted QUIC session.
	// It is the responsibility of the function to return a valid Requester.
	// If Requester returns an error, the server will close the QUIC session.
	// If nil, http3.Open will be used.
	Requester func(quic.EarlySession, Settings) (Requester, error)
```

Second, add a flag to `Server` to enable the [extended CONNECT method](https://datatracker.ietf.org/doc/html/rfc8441) (necessary for WebTransport or WebSockets):

```go
	// Enable support for extended CONNECT method.
	// If set to true, the server will support CONNECT requests with a :path and :protocol header.
	EnableConnectProtocol bool
```

### `http3.Requester`

A `Requester` is responsible for providing HTTP requests to the server.

The default implementation wraps a QUIC session and handles H3 framing, request and response body streaming, and translation to/from `http` semantics. It accepts streams and datagrams, (de)multiplexing them to the relevant H3 request sessions.

The `http3` package would provide a default `Accept()` func to create a `Requester` from a QUIC session. It sets up initial state, opens the unidirectional control stream, and sends the H3 settings frame.

A WebTransport extension, for example, could override `(Server).Requester` to provide a WebTransport-aware `Requester`, and dispatch incoming WT streams and datagrams to the appropriate H3 session.

```go
// Requester represents a server-side HTTP/3 connection.
// Implementations may implement other interfaces.
type Requester interface {
	AcceptHTTP() (*http.Request, http.ResponseWriter, error)
	io.Closer
}

// Accept takes a QUIC session and HTTP/3 settings, and returns a Requester.
// It opens the control stream and sends the initial H3 settings frame,
// returning an error if either fail. The returned Requestor is ready to use.
func Accept(session quic.EarlySession, settings Settings) (Requester, error) {
	...
}
```

A `Requester` can be extended to support additional features, e.g. `interface Pusher { ... }`.

### `http3.Conn`

Internally, the default implementation of `Requester` sits on top of `http3.Conn`, which combines a `quic.EarlySession` with a QPACK handler and some other state. Both H3 client and server connections would use `Conn`.

It can be created from a quic.EarlySession via `http3.Open(quic.EarlySession, http3.Settings) (Conn, error)`.

Internally, a `Conn` holds:

- `quic.EarlySession`
- `qpack.Decoder`
- Control stream
- Peer control stream
- Settings
- Peer settings

```go
type Conn interface {
	AcceptStream(ctx) (http3.Stream, error)
	AcceptUniStream(ctx) (http3.ReceiveStream, error)
	OpenStream() (http3.Stream, error)
	OpenUniStream(streamType uint64) (http3.SendStream, error)

	DecodeHeaders(io.Reader) (http.Header, error)

	PeerSettings() (http3.Settings, error)

	io.Closer
}

// Necessary between Requester and Conn?
type ServerConn interface {
	AcceptRequest(ctx) (http3.Request, error)
	Conn
}
```

### `http3.Stream`

- the parent http3.Conn
- a QUIC stream

```go
type Stream {
	FrameReader
	FrameWriter
	io.Closer
}

type ReceiveStream interface {
	StreamTyper
	FrameReader
}

type SendStream interface {
	StreamTyper
	FrameWriter
	io.Closer
}

type StreamTyper interface {
	StreamType() uint64
}

type FrameReader interface {
	ReadFrame() (http3.Frame, error)
	Skip(int64) error
}

type FrameWriter interface {
	WriteFrame(http3.Frame) error
}
```

### `http3.Frame`

- There can be an `http3.UnknownFrame` which parses only the frame type and stops.

```go
type Frame interface {
	FrameType() uint64
	Payload() io.ReadCloser
}
```

`Frame` may also implement `FrameLength() protocol.ByteCount` and `io.WriterTo`.

`Payload` may also implement `Size() protocol.ByteCount`.

Concrete implementations of http3.Frame MAY implement other methods.

### `http3.Request`

An [`http3.Request`](https://www.ietf.org/archive/id/draft-ietf-quic-http-34.html#name-http-message-exchanges) holds:

- an `http3.Stream`
- headers
- trailers
- authority
- method
- extended connect bit?
- request body

It can be created from an http3.Stream via:

- http3.NewRequest(http3.Stream, http.Header) (http3.Request, error)

An `http3.Server` can handle an `http3.Request` with:

- (Server).ServeHTTP3(http3.Request) error

### Misc

- `http3.Settings` is a `map[uint64]uint64` with some helper methods.
- `http3.Frame` is `interface { FrameType() uint64 }`
- `io.Reader` is a `Frame`, etc.
