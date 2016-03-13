// TODO: add some copyright
package main

import (
	"crypto/tls"
	go_http2 "golang.org/x/net/http2"
	"log"
	"net"
)

// Flags is a bitmask of HTTP/2 flags.
// It's used to interpret frame types.
type Flags uint8

// Has reports whether f contains all flags in v.
func (f Flags) Has(v Flags) bool {
	return (f & v) == v
}

type FrameType uint8

var frameName = map[FrameType]string{
	go_http2.FrameData:         "data",
	go_http2.FrameHeaders:      "headers",
	go_http2.FramePriority:     "priority",
	go_http2.FrameRSTStream:    "rst_stream",
	go_http2.FrameSettings:     "settings",
	go_http2.FramePushPromise:  "push_promise",
	go_http2.FramePing:         "ping",
	go_http2.FrameGoAway:       "goaway",
	go_http2.FrameWindowUpdate: "window_update",
	go_http2.FrameContinuation: "continuation",
}

var flagName = map[FrameType]map[Flags]string{
	FrameData: {
		go_http2.FlagDataEndStream: "END_STREAM",
		go_http2.FlagDataPadded:    "PADDED",
	},
	FrameHeaders: {
		go_http2.FlagHeadersEndStream:  "END_STREAM",
		go_http2.FlagHeadersEndHeaders: "END_HEADERS",
		go_http2.FlagHeadersPadded:     "PADDED",
		go_http2.FlagHeadersPriority:   "PRIORITY",
	},
	FrameSettings: {
		go_http2.FlagSettingsAck: "ACK",
	},
	FramePing: {
		go_http2.FlagPingAck: "ACK",
	},
	FrameContinuation: {
		go_http2.FlagContinuationEndHeaders: "END_HEADERS",
	},
	FramePushPromise: {
		go_http2.FlagPushPromiseEndHeaders: "END_HEADERS",
		go_http2.FlagPushPromisePadded:     "PADDED",
	},
}

func (t FrameType) String() string {
	if s, ok := frameName[t]; ok {
		return s
	}
	return fmt.Sprintf("UNKNOWN frame type: %d", t)
}

// A FrameHeader is the 9 byte header of all HTTP/2 frames.
type FrameHeader struct {
	// Type is the 1 byte frame type. There are 10 standard frame types.
	// Eg: one of the constants in go_http2, like FrameData.
	Type FrameType

	// Flags are the 1 byte of 8 potential bit flags per frame.
	// Eg: one of the constants in go_htt2, like FlagDataEndStream.
	Flags Flags

	// Length is the length of the frame, not including the 9 byte header.
	// The max size is one byte less than 16MB, but only frames upto 16KB
	// are allowed without peer agreement.
	Length uint32

	// StreamID is which stream this frame is for. Certain frames are not
	// stream-specific, in which case this field is 0.
	StreamID uint32
}

var frameHeaderLen = 9

func readFrameHeader(buf []byte, r io.Reader) (FrameHeader, error) {
	_, err := io.ReadFull(r, buf[:frameHeaderLen])
	if err != nil {
		return FrameHeader{}, err
	}
	return FrameHeader{
		Length:   (uint32(buf[0])<<16 | uint32(buf[1])<<8 | uint32(buf[2])),
		Type:     FrameType(buf[3]),
		Flags:    Flags(buf[4]),
		StreamID: binary.BigEndian.Uint32(buf[5:]) & (1<<31 - 1),
	}, nil
}

// [FrameHeader HEADERS flags=END_STREAM|ox2|END_HEADERS stream=1 len=17]
func (h FrameHeader) String() string {
	var buf bytes.Buffer
	buf.WriteString("[FrameHeader ")
	buf.WriteString(h.Type.String())
	if h.Flags != 0 {
		buf.WriteString(" flags=")
		set := 0
		for i := uint8(0); i < 8; i++ {
			if h.Flags&(1<<i) == 0 {
				continue
			}
			set++
			if set > 1 {
				buf.WriteByte('|')
			}
			name := flagName[h.Type][Flags(1<<i)]
			if name != "" {
				buf.WriteString(name)
			} else {
				fmt.Fprintf(&buf, "0x%x", 1<<i)
			}
		}
	}
	if h.StreamID != 0 {
		fmt.Fprintf(&buf, " stream=%d", h.StreamID)
	}
	fmt.Fprintf(&buf, " len=%d]", h.Length)
	return buf.String()
}

func main() {
	cert, err := tls.LoadX509KeyPair("/tmp/nginx.crt", "/tmp/nginx.key")
	check(err)
	ln, err := net.Listen("tcp", "localhost:4430")
	check(err)
	log.Printf("serving on 4430")
	tln := tls.NewListener(ln, &tls.Config{
		NextProtos:   []string{"h2"},
		Certificates: []tls.Certificate{cert},
	})
	for {
		conn, err := tln.Accept()
		if err != nil {
			log.Print(err)
			continue
		}
		tc := conn.(*tls.Conn)
		if err := tc.Handshake(); err != nil {
			conn.Close()
			log.Printf("Handshake error: %v", err)
			continue
		}
		log.Printf("%v connected with state: %+v", conn.RemoteAddr(), tc.ConnectionState())
		conn.Close()
	}
}

func check(err error) {
	if err != nil {
		panic(err.Error())
	}
}
