// TODO: Add copyright

package http2

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"errors"
	"fmt"
	go_http2 "golang.org/x/net/http2"
	"golang.org/x/net/http2/hpack"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
)

type Transport struct {
	Fallback http.RoundTripper
}

type clientConn struct {
	tconn *tls.Conn
	bw    *bufio.Writer
	br    *bufio.Reader
	fr    *go_http2.Framer

	// first write error
	werr error

	// Buffer for hapck encoder.
	hbuf bytes.Buffer
	henc *hpack.Encoder

	// Settings from peer/server
	maxFrameSize uint32
}

type stickyErrWriter struct {
	w   io.Writer
	err *error
}

func (sew stickyErrWriter) Write(p []byte) (n int, err error) {
	if *sew.err != nil {
		return n, *sew.err
	}
	n, err = sew.w.Write(p)
	sew.err = &err
	return
}

func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL.Scheme != "https" {
		if t.Fallback == nil {
			return nil, errors.New("Unsupported scheme and no fallback.")
		}
		return t.Fallback.RoundTrip(req)
	}
	// We know it's https, so open up a tls connection to the host in the url.
	// The host header from url.Host overrides the request.Host.
	// The url.Host also includes port.
	host, port, err := net.SplitHostPort(req.URL.Host)
	if err != nil {
		host = req.URL.Host
		port = "443"
	}
	config := &tls.Config{
		ServerName: host,
		NextProtos: []string{go_http2.NextProtoTLS},
	}
	tConn, err := tls.Dial("tcp", host+":"+port, config)
	if err != nil {
		return nil, err
	}
	if err := tConn.Handshake(); err != nil {
		return nil, err
	}
	// We don't need this with insecurySkipTLSVerify
	if err := tConn.VerifyHostname(config.ServerName); err != nil {
		return nil, err
	}
	state := tConn.ConnectionState()
	if state.NegotiatedProtocol != go_http2.NextProtoTLS || !state.NegotiatedProtocolIsMutual {
		// TODO: fall back
		return nil, fmt.Errorf("Couldn't negotiate http2: %+v", state)
	}
	if _, err := tConn.Write([]byte(go_http2.ClientPreface)); err != nil {
		return nil, err
	}
	log.Printf("Negotiation passed")

	cc := &clientConn{
		tconn: tConn,
	}
	cc.bw = bufio.NewWriter(stickyErrWriter{tConn, &cc.werr})
	cc.br = bufio.NewReader(tConn)
	cc.fr = go_http2.NewFramer(cc.bw, cc.br)
	cc.henc = hpack.NewEncoder(&cc.hbuf)

	// TODO: Better options
	cc.fr.WriteSettings()
	cc.bw.Flush()
	if cc.werr != nil {
		return nil, err
	}
	f, err := cc.fr.ReadFrame()
	if err != nil {
		return nil, err
	}

	switch f := f.(type) {
	case *go_http2.SettingsFrame:
		// TODO: We'll need to remember these:
		// eg:
		// 2016/03/12 15:45:27 Setting: [MAX_CONCURRENT_STREAMS = 100]
		// 2016/03/12 15:45:27 Setting: [INITIAL_WINDOW_SIZE = 1048576]
		// 2016/03/12 15:45:27 Setting: [MAX_FRAME_SIZE = 16384]
		f.ForeachSetting(func(s go_http2.Setting) error {
			switch s.ID {
			case go_http2.SettingMaxFrameSize:
				cc.maxFrameSize = s.Val
			default:
				log.Printf("Setting: %v", s)
			}
			return nil
		})
	}

	// we send: HEADERS[+CONTINUATION] + (DATA?)
	hdrs := cc.encodeHeaders(req)
	first := true
	streamID := cc.nextStreamID()
	hasBody := false
	for len(hdrs) > 0 {
		chunk := hdrs
		if len(chunk) > int(cc.maxFrameSize) {
			chunk = chunk[:cc.maxFrameSize]
		}
		hdrs = hdrs[len(chunk):]
		endHeaders := len(hdrs) == 0
		if first {
			cc.fr.WriteHeaders(go_http2.HeadersFrameParam{
				StreamID:      streamID,
				BlockFragment: chunk,
				EndHeaders:    endHeaders,
				// Is there a request body? for GET it's no
				EndStream: !hasBody,
			})
			first = false
		} else {
			cc.fr.WriteContinuation(streamID, endHeaders, chunk)
		}
	}

	cc.bw.Flush()
	if cc.werr != nil {
		return nil, cc.werr
	}

	// server sends: HEADERS[+CONTINUATION]
	f, err = cc.fr.ReadFrame()
	if err != nil {
		return nil, err
	}
	log.Printf("Got frame %+v", f)

	return nil, errors.New("TODO")
}

func (cc *clientConn) encodeHeaders(req *http.Request) []byte {
	cc.hbuf.Reset()
	cc.writeHeader(":method", req.Method)
	cc.writeHeader(":scheme", "https")
	// cc.writeHeader(":authority", req.Host)
	cc.writeHeader(":path", req.URL.Path)
	for k, vv := range req.Header {
		for _, v := range vv {
			cc.writeHeader(strings.ToLower(k), v)
		}
	}
	return cc.hbuf.Bytes()
}

func (cc *clientConn) writeHeader(name, val string) {
	// This writes to the buffer, not conn
	cc.henc.WriteField(hpack.HeaderField{Name: name, Value: val})
}

func (cc *clientConn) nextStreamID() uint32 {
	return 1
}
