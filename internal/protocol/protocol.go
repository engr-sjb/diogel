/*
	Copyright (c) 2025 Stephen Jersuit Benyah
	Licensed under the Repo-Only Non-Commercial & No-Derivatives License with Anti-Training Clause (RONCND-AT) v1.0.
	See LICENSE and CONTRIBUTION_LICENSE_AGREEMENT.md in repository root.
	Prohibited: copying, reuse, redistribution, or use as training data for machine learning/AI.
*/

package protocol

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"log"

	"io"

	"github.com/engr-sjb/diogel/internal/message"
	"github.com/engr-sjb/diogel/internal/serialize"
)

var byteOrder = binary.BigEndian

type version uint8

const (
	// version1 is the version 1 of the protocol.
	//notice: Adjust this v1HeaderSize accordingly if you add more fields for the frame.

	v1                  version = 1
	v1HeaderVersionSize uint8   = 1 // This is the size of the protocol version to be read.
	v1HeaderPayloadSize uint8   = 4 // This is the size of the payload to be read.
	v1HeaderSize        uint8   = v1HeaderVersionSize + v1HeaderPayloadSize

	maxLength = 16 << 20 //16KiB	// TODO: check this max length for header.
	// op = 1024 * 16240

)

type Protocol interface {
	Version() version
	DoServerHandshake(remotePeerConn io.ReadWriter, localPublicKey []byte) (remotePublicKey []byte, err error)
	DoClientHandshake(remotePeerConn io.ReadWriter, localPublicKey []byte) (remotePublicKey []byte, err error)
	ReadFrame(r io.Reader, rf *Frame) error
	WriteFrame(w io.Writer, wf *Frame) error
}

type protocol struct {
	serialize serialize.Serializer
	// TODO: might have to bring cCrypto in here for handshake
}

var _ Protocol = (*protocol)(nil)

func NewProtocol(s serialize.Serializer) *protocol {
	return &protocol{
		serialize: s,
	}

}

func (p protocol) DoServerHandshake(remotePeerConn io.ReadWriter, localPublicKey []byte) (remotePublicKey []byte, err error) {
	// TODO: I can pass in a remote key slice to fill with their remote key rather than returning a slice. Not sure yet.

	//------ Receive their public key.
	remotePublicKey, err = receivePublicKey(remotePeerConn)
	if err != nil {
		return nil, err
	}

	//------ Send our public key.
	err = sendPublicKey(remotePeerConn, localPublicKey)
	if err != nil {
		return nil, err
	}

	return remotePublicKey, nil
}

func (p protocol) DoClientHandshake(remotePeerConn io.ReadWriter, localPublicKey []byte) (remotePublicKey []byte, err error) {
	// TODO: I can pass in a remote key slice to fill with their remote key rather than returning a slice. Not sure yet.

	/*
		- send public key; why? for them to use to encrypt further handshake msgs they send to me.
		- receive their public key; why? for to use to encrypt further handshake msgs i send to them

		- first 4 bytes is the size of localPublicKey
		- the write the localPublicKey of the size.
	*/

	//------ Send our public key.
	err = sendPublicKey(remotePeerConn, localPublicKey)
	if err != nil {
		return nil, err
	}

	//------ Receive their public key.
	remotePublicKey, err = receivePublicKey(remotePeerConn)
	if err != nil {
		return nil, err
	}

	return remotePublicKey, nil
}

func sendPublicKey(remotePeerConn io.ReadWriter, localPublicKey []byte) error {
	// size := 4 //TODO: pick a better name for the number of bytes that hold the length size of localPublicKey.

	localPublicKeySize := len(localPublicKey)

	buf := make([]byte, (int(v1HeaderPayloadSize) + localPublicKeySize))
	byteOrder.PutUint32(buf[:int(v1HeaderPayloadSize)], uint32(localPublicKeySize))
	copy(buf[int(v1HeaderPayloadSize):], localPublicKey)

	_, err := remotePeerConn.Write(buf)
	if err != nil {
		return fmt.Errorf(
			"failed to send our public key to remote peer: %w",
			err,
		)
	}

	return nil
}

func receivePublicKey(remotePeerConn io.ReadWriter) ([]byte, error) {
	// var size int = 4 //TODO: pick a better name for the number of bytes that hold the length size of localPublicKey. and should be a package variable.

	payloadBufSize := make([]byte, v1HeaderPayloadSize)
	_, err := io.ReadFull(
		remotePeerConn,
		payloadBufSize,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to receive public key from remote peer: %w",
			err,
		)
	}

	remotePublicKeySize := byteOrder.Uint32(payloadBufSize)

	remotePublicKey := make([]byte, remotePublicKeySize)
	_, err = io.ReadFull(
		io.LimitReader(remotePeerConn, int64(remotePublicKeySize)),
		// payloadBuf,
		remotePublicKey,
	)
	if err != nil {
		log.Println(err)
		return nil, fmt.Errorf(
			"failed to receive public key from remote peer: %w",
			err,
		)
	}

	return remotePublicKey, err
}

type Payload struct {
	// we can have anything in here to be sent over the wire
	Msg message.Msg // TODO: might have to create a msg type for msg in payload rather any. not sure but try and see. run test
}

type Frame struct {
	Version version
	Payload
}

func (p protocol) ReadFrame(r io.Reader, rf *Frame) error {
	/*
		header
		- byte[0]   |1bytes| = version
		- byte[1:5] |4bytes| = msgSize

		read this number of msgSize bytes from the reader to get the msg

		the payload is a gob encoding of various Msg structs

		This is the reading framing protocol v1.
		- read the headerBuf of v1HeaderSize from the reader.
		- 1B of headerBuf for version from the header.
		- read 4byte for the msg(payload) size from header.
		- now read msg(payload) size from the reader to get msg.
		- now use gob to serialize it into a msg type which will later be switch type cast on.
	*/

	//TODO: Might have to decrypt data before with a Key(mainly a public key in this case.) or maybe do it after writing to the rf here. so its data is is decrypted outside. Not sure yet.

	headerBuf := make([]byte, v1HeaderSize)
	_, err := io.ReadFull(r, headerBuf)
	if err != nil {
		return err
	}

	rf.Version = version(headerBuf[0])
	msgSize := byteOrder.Uint32(headerBuf[1:5])

	err = p.serialize.Decode(
		io.LimitReader(r, int64(msgSize)),
		&rf.Payload,
	)
	if err != nil {
		return err
	}

	return nil
}

func (p protocol) WriteFrame(w io.Writer, wf *Frame) error {
	var payloadBuf bytes.Buffer
	if err := p.serialize.Encode(&payloadBuf, wf.Payload); err != nil {
		return err
	}

	totalFrameSize := int(v1HeaderSize) + payloadBuf.Len()

	buf := make([]byte, totalFrameSize)
	buf[0] = byte(wf.Version)
	byteOrder.PutUint32(buf[1:5], uint32(payloadBuf.Len()))
	copy(buf[v1HeaderSize:], payloadBuf.Bytes())

	//TODO: Might have to encrypt data before with a Key(mainly a public key in this case.) or maybe do it before passing the wf here. so its data is already encrypted. Not sure yet.

	_, err := w.Write(buf)
	if err != nil {
		return err
	}

	return nil
}

func (p *protocol) Version() version {
	return v1
}
