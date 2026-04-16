package peererrors

import "github.com/engr-sjb/diogel/internal/features"

type Scope uint8

const (
	// ScopeInternalPeer is a error that should only be known to the
	// internal system. it shouldn’t NEVER be sent or displayed to the local or
	// remote peer. It can be used for logs, debugging, etc.
	ScopeInternalPeer Scope = iota
	// ScopeLocalPeer is a error that can be displayed to the user(local peer).
	ScopeLocalPeer
	// ScopeRemotePeer is a error that can be sent to the remote peer.
	ScopeRemotePeer
)

type Code uint16

const (
	CodeTodo Code = iota
)

const (
	//Network: 1000+
	ErrInternalDB Code = 1000 + iota
)

const (
	//Auth: 2000+
	ErrBadRequest Code = 2000 + iota
)

const (
	//Internal: 5000+
	ErrErasureCoding Code = 5000 + iota
)

// func (c Code) Error() string {
// 	return c.String()
// }
// func (c Code) IsInternal() bool {
// 	return c == ErrInternalDB
// }

func (c Code) String() string {
	switch c {
	// case ErrInternalDB:
	// 	return "internal database error."
	}
	return "unknown error."

}

type PeerError struct {
	scope   Scope
	code    Code
	message string
	// internalMessage string
	err     error
	featLoc features.FeatureLocation // The feature location where the error occurred. Helps with debugging fast.
}

func New(scope Scope, code Code, message string, err error, featLoc features.FeatureLocation) error {
	//TODO: I think we need to add a user message and a system or dev message for the engineers.
	return &PeerError{
		scope:   scope,
		message: message,
		err:     err,
		featLoc: featLoc,
	}
}

func (e PeerError) Error() string {
	return e.message
}

func (e PeerError) Message() string {
	return e.message
}

func (e PeerError) Code() Code {
	return e.code
}
func (e PeerError) Scope() Scope {
	return e.scope
}
func (e PeerError) FeatLoc() features.FeatureLocation {
	return e.featLoc
}
