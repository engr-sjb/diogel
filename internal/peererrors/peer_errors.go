package peererrors

import "github.com/engr-sjb/diogel/internal/features"

type Code uint8

const (
	// CodeInternalPeerError is a error that should only be known to the
	// internal system. it shouldn’t NEVER be sent or displayed to the local or
	// remote peer. It can be used for logs, debugging, etc.
	CodeInternalPeerError Code = iota

	// CodeLocalPeerError is a error that can be displayed to the user(local peer).
	CodeLocalPeerError
	// CodeRemotePeerError is a error that can be sent to the remote peer.
	CodeRemotePeerError
)

type peerError struct {
	code    Code
	message string
	err     error
	featLoc features.FeatureLocation // The feature location where the error occurred. Helps with debugging fast.
}

func New(code Code, message string, err error, featLoc features.FeatureLocation) error {
	//TODO: I think we need to add a user message and a system or dev message for the engineers.
	return &peerError{
		code:    code,
		message: message,
		err:     err,
		featLoc: featLoc,
	}
}

func (e peerError) Error() string {
	return e.message
}

func (e peerError) Message() string {
	return e.message
}

func (e peerError) Code() Code {
	return e.code
}
func (e peerError) FeatLoc() features.FeatureLocation {
	return e.featLoc
}
