package protocol

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

// errBadOption says that a thing that may be missing had a tag that is
// neither missing nor there.
var errBadOption = errors.New("protocol: the tag of something that may be missing is wrong")

// The kinds of the messages the daemon sends.
type ServerKind int

// The kinds of the messages of the daemon.
const (
	// ServerUnknown is a message of a kind this client does not read.
	ServerUnknown ServerKind = iota
	// ServerKindWelcome is the answer of the daemon to the handshake.
	ServerKindWelcome
	// ServerControl is a named control message: the handshake's answer,
	// the snapshot.
	ServerControl
	// ServerResponse is a piece of the answer to a request.
	ServerResponse
	// ServerSurface is the surface of the pane in front.
	ServerSurface
	// ServerSurfacePatch is a change of the surface.
	ServerSurfacePatch
	// ServerSnapshot is the snapshot of the session.
	ServerSnapshot
	// ServerError is an error the window must show.
	ServerError
	// ServerNotification is a message that says something happened.
	ServerNotification
	// ServerWindowTitle is the title of the window.
	ServerWindowTitle
)

// ServerMessage is a message the daemon sends.
type ServerMessage struct {
	// Kind tells what the message is.
	Kind ServerKind
	// Control is the named control message, when the kind is
	// ServerControl.
	Control *Control
	// Response is a piece of the answer to a request.
	Response *ResponseChunk
	// Surface is the surface of the pane in front.
	Surface *SurfaceFrame
	// Snapshot is the snapshot of the session.
	Snapshot *Snapshot
	// Error is the error the window must show.
	Error *ShellError
	// Notification is the message that says something happened.
	Notification *Notification
	// WindowTitle is the title of the window.
	WindowTitle string
}

// Control is a named control message of the endpoint protocol.
type Control struct {
	// Kind is the name of the control message.
	Kind string
	// Data is the JSON it carries.
	Data string
}

// ResponseChunk is one piece of the answer to a request.
type ResponseChunk struct {
	// BootID is the boot the answer belongs to.
	BootID string
	// RequestID is the request the answer is for.
	RequestID string
	// FinalChunk tells that no more pieces are coming.
	FinalChunk bool
	// Data is the JSON of the piece.
	Data string
}

// ShellError is an error the window must show.
type ShellError struct {
	// Message is what went wrong.
	Message string
	// Kind is the sort of the error.
	Kind string
}

// Error makes the error say what went wrong.
func (e *ShellError) Error() string { return e.Message }

// Notification is something that happened that the window shows.
type Notification struct {
	// Kind is what happened.
	Kind string
	// Title is the title of the message.
	Title string
	// Body is the message itself.
	Body string
}

// The kinds of the control messages of the endpoint protocol.
const (
	// ControlHello is the handshake of the client.
	ControlHello = "endpoint.hello.v1"
	// ControlWelcome is the answer of the daemon.
	ControlWelcome = "endpoint.welcome.v1"
	// ControlSnapshot is the snapshot of the session.
	ControlSnapshot = "shell.snapshot.v1"
)

// The kinds of the messages of the daemon that this client reads.
const (
	serverWelcomeVariant          = 0
	serverWindowTitleVariant      = 6
	serverClientShellSnapshotV    = 12
	serverPaneSurfaceVariant      = 13
	serverSemanticNotificationV   = 14
	serverClientShellErrorVariant = 15
	serverEndpointResponseVariant = 18
	serverPaneSurfacePatchVariant = 19
	serverEndpointControlVariant  = 20
)

// DecodeServerMessage reads a message of the daemon.
func DecodeServerMessage(data []byte) (*ServerMessage, error) {
	r := NewReader(data)
	variant, err := r.Variant()
	if err != nil {
		return nil, err
	}
	switch variant {
	case serverEndpointControlVariant:
		kind, err := r.Str()
		if err != nil {
			return nil, err
		}
		payload, err := r.Str()
		if err != nil {
			return nil, err
		}
		return &ServerMessage{
			Kind:    ServerControl,
			Control: &Control{Kind: kind, Data: payload},
		}, nil
	case serverEndpointResponseVariant:
		chunk := &ResponseChunk{}
		if chunk.BootID, err = r.Str(); err != nil {
			return nil, err
		}
		if chunk.RequestID, err = r.Str(); err != nil {
			return nil, err
		}
		if chunk.FinalChunk, err = r.Bool(); err != nil {
			return nil, err
		}
		bytes, err := r.ByteSlice()
		if err != nil {
			return nil, err
		}
		chunk.Data = string(bytes)
		return &ServerMessage{Kind: ServerResponse, Response: chunk}, nil
	case serverPaneSurfaceVariant:
		frame, err := decodeSurfaceFrame(r)
		if err != nil {
			return nil, err
		}
		return &ServerMessage{Kind: ServerSurface, Surface: frame}, nil
	case serverClientShellSnapshotV:
		// The snapshot of the client shell arrives as JSON in a box.
		kind, err := r.Str()
		if err != nil {
			return nil, err
		}
		payload, err := r.Str()
		if err != nil {
			return nil, err
		}
		if kind == ControlSnapshot {
			snapshot, err := DecodeSnapshot(payload)
			if err != nil {
				return nil, err
			}
			return &ServerMessage{Kind: ServerSnapshot, Snapshot: snapshot}, nil
		}
		return &ServerMessage{Kind: ServerControl, Control: &Control{Kind: kind, Data: payload}}, nil
	case serverClientShellErrorVariant:
		message, err := r.Str()
		if err != nil {
			return nil, err
		}
		return &ServerMessage{Kind: ServerError, Error: &ShellError{Message: message}}, nil
	case serverSemanticNotificationV:
		notification, err := decodeNotification(r)
		if err != nil {
			return nil, err
		}
		return &ServerMessage{Kind: ServerNotification, Notification: notification}, nil
	case serverWindowTitleVariant:
		title, err := r.Str()
		if err != nil {
			return nil, err
		}
		return &ServerMessage{Kind: ServerWindowTitle, WindowTitle: title}, nil
	}
	return &ServerMessage{Kind: ServerUnknown}, nil
}

// decodeNotification reads a notification.
func decodeNotification(r *Reader) (*Notification, error) {
	notification := &Notification{}
	var err error
	if notification.Kind, err = r.Str(); err != nil {
		return nil, err
	}
	if notification.Title, err = r.Str(); err != nil {
		return nil, err
	}
	if notification.Body, err = r.Str(); err != nil {
		return nil, err
	}
	return notification, nil
}

// WriteFrame writes a frame: the length of the message, and the message.
//
// The length is four bytes, little endian, and says how many bytes follow.
func WriteFrame(w io.Writer, payload []byte) error {
	if len(payload) > maxFrameSize {
		return fmt.Errorf("protocol: the frame of %d bytes is too big", len(payload))
	}
	var header [4]byte
	binary.LittleEndian.PutUint32(header[:], uint32(len(payload)))
	if _, err := w.Write(header[:]); err != nil {
		return err
	}
	_, err := w.Write(payload)
	return err
}

// maxFrameSize is the biggest frame the daemon reads.
const maxFrameSize = 64 << 20

// ReadFrame reads a frame: the length of the message, and the message.
func ReadFrame(r io.Reader) ([]byte, error) {
	var header [4]byte
	if _, err := io.ReadFull(r, header[:]); err != nil {
		return nil, err
	}
	length := binary.LittleEndian.Uint32(header[:])
	if length > maxFrameSize {
		return nil, fmt.Errorf("protocol: the frame of %d bytes is too big", length)
	}
	payload := make([]byte, length)
	if _, err := io.ReadFull(r, payload); err != nil {
		return nil, err
	}
	return payload, nil
}
