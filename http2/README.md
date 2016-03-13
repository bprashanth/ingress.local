Create a normal http client but modify the roundtripper interface to support http2.
```
type RoundTripper interface {
    RoundTrip(*Request) (*Response, error)
}
```

Spec:
* We need to negotiate up to http2 during the tls handshale. This requires a custom tls config passed to the Dial function:

```
func Dial(network, addr string, config *Config) (*Conn, error)

* Connects to addr (syn, ack etc)
```
* The next protocol is *not* https, but h2. Servers advertise this as a supported protocol.
```
    // NextProtoTLS is the NPN/ALPN protocol negotiated during
    // HTTP/2's TLS setup.
    NextProtoTLS = "h2"
```
* The config needs to specify a servername for the tls handshake. This doens't include port, and must match both the host in the URL (since we're using that host in dial) and the cert (on the server) for a successful handshake.
* This client is given a url, which has a scheme that might be gopher, ftp or whaterver. The http2 client only recognizes https, so anything else needs to use a fallback client.
* Http2 requests are pipelined, but we can start with 1 connection per request.
* Negotiation happens during the handshake, which happens after Dial.
```
func (c *Conn) Handshake() error
```
* After a successful handshake, we can optionally invoke VerifyHostname which checks the certificate chain.
```
func (c *Conn)VerifyHostname (host string) error
```
* After handshake connectionState is populated with negotiated protocol:
```
2016/03/12 15:22:13 {Version:771 HandshakeComplete:true DidResume:false CipherSuite:49199 NegotiatedProtocol:h2 NegotiatedProtocolIsMutual:true ServerName: PeerCertificates:[junk junk junk] VerifiedChains:[[junk][junk][junk]] SignedCertificateTimestamps:[junkj]] OCSPResponse:[] TLSUnique:[junk]}
```
* Once we get a connection we need to start writing frames, but before doing so we need to write out a Preface
```
    // ClientPreface is the string that must be sent by new
    // connections from clients.
    ClientPreface = "PRI * HTTP/2.0\r\n\r\nSM\r\n\r\n"
```
Preface is defined in: https://http2.github.io/http2-spec/#rfc.section.3.5
* To start sending frames, we need to make a "Framer", frames can get written into a bufio instead of directly to the conn.
* Settings frame: There are several write methods on a Framer, write setting, write settings ack etc. Start with Writing settings. The 2 sides need to negotiate a frame size. Sending empty settings negotiates defaults. At this point the server sends back a frame. Frames have types. This is a settings frame.
```
2016/03/12 15:45:27 Setting: [MAX_CONCURRENT_STREAMS = 100]
2016/03/12 15:45:27 Setting: [INITIAL_WINDOW_SIZE = 1048576]
2016/03/12 15:45:27 Setting: [MAX_FRAME_SIZE = 16384]
2
```
- appspot.com says that we can have a max of 100 HTTP requests in flight for 1 tls connection. This needs to be tracked in the client.
- Default options have picked the initial window size and frame size.
* Now the client/server is in an idle state and we cat start exchanging frames. These are just HTTP HEADERS, CONTINUATIONS, DATA(POST data). Continuations are just chunked headers, since we can only write some limt per frame.
* A HEADERS frame:
```
type HeadersFrameParam struct {
    // StreamID is the required Stream ID to initiate.
    StreamID uint32
    // BlockFragment is part (or all) of a Header Block.
    BlockFragment []byte

    // EndStream indicates that the header block is the last that
    // the endpoint will send for the identified stream. Setting
    // this flag causes the stream to enter one of "half closed"
    // states.
    EndStream bool

    // EndHeaders indicates that this frame contains an entire
    // header block and is not followed by any
    // CONTINUATION frames.
    EndHeaders bool

    // PadLength is the optional number of bytes of zeros to add
    // to this frame.
    PadLength uint8

    // Priority, if non-zero, includes stream priority information
    // in the HEADER frame.
    Priority PriorityParam
}
```
* We need to convert HTTP request headers into Headers frame.
- Stream id: every request has an id that goes up.
- block framgment: HPACK encoded header block.
- End headers: set on last continutaion.
- Padlength: obscurity
- Priority: of this request
* The headers are encoded using HPACK. The encoder writes data to an io.Writer but it can't go straight out on the connection because if the request headers end up being more than 16k (max frame size) we need to chunk it into continuation frames.
* Encoding of headers happens through a HeaderField, but we need to pay attention to the ordering. Magic priority headers that need to go first include:
```
The :method pseudo-header field includes the HTTP method ([RFC7231], Section 4).

The :scheme pseudo-header field includes the scheme portion of the target URI ([RFC3986], Section 3.1).

:scheme is not restricted to http and https schemed URIs. A proxy or gateway can translate requests for non-HTTP schemes, enabling the use of HTTP to interact with non-HTTP services.

The :authority pseudo-header field includes the authority portion of the target URI ([RFC3986], Section 3.2). The authority MUST NOT include the deprecated userinfo subcomponent for http or https schemed URIs.
```
* Assuming the headers are all written out, we've made a first request. Http2 is flow controlled, so the next server frame will be a "window update".


Key features of HTTP2:
HTTP accumulated hacks, special cases for working around browser implementationsi (6 connections per host, each gets slow start). Pipelining supported in 1.1, ask for a bunch of things and get them back in order, but mitm proxies just choke on it.
* Chrome + GFE experiments => both sides opt in to an all encrypted binary protocol.
* Different types of frames: DATA, HEADER, CONTINUATION
* Entire frame:
[frame length] [1 byte for what this frame is (eg: HEADER)] [end stream, end headers] [stream id] [frame length bytes of frame datai -- HPACK compressed]
* Hpack encoding in the spec: google did static analysis on web traffic and came up with the smallest way to encode common HTTP idioms, using huffman encoding and a static lookup tablei (https://tools.ietf.org/html/rfc7541).
* Connection is stateful, sending cookies and UA is per connection (not per stream).
* Frame types:
    - data: post body,response body
    - headers + continuations: can't be interleaved, like all the other frames
    - settings: upgrade, flow control rates, max size of packets etc
    - rst_stream
    - ping: detects idle tcp conns
    - goaway: apache has keepalive: 30s and at browser sends at 29, and rst from apache comes down at the same time (doesn't tcp take care of this?).
    - window_update: everything is flow controlled on each stream (look this up).
* Upgrading: handshake, negotiate after cipher suites. Cannot currently upgrade http.
* Start by reading headers, then read Data frame. Each data frame is associated with a stream. We don't need to kill and entire connection on a single stream error.
* Each connection gets 3/4 goroutines:
    - reading from socket: readFrameHeader waiting for 9 bytes to show up, and then reads the rest of the frame into memory and sends it to serve goroutine.
    - write to socket: same as read, but invokes writeFrameHeader.
    - serve: maintains connection state.
        Http handler, Http handler ... (serveMux)
goroutines are 4k, 8k, 2k something.
* Either side can initiate streams, so the ids need to be even from client and odd from server. The server can issue a response for a request that hasn't been made yet with the odd stream ids (/ also gets request for foo.css+foo.css).
