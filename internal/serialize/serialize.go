package serialize

import (
	"encoding/gob"
	"io"
)

type Serializer interface {
	Encode(io.Writer, any) error
	Decode(r io.Reader, p any) error
}

type serialize struct{}

func New() Serializer {
	return &serialize{}
}

func (serialize) Encode(w io.Writer, p any) error {
	return gob.NewEncoder(w).Encode(p)
}

func (serialize) Decode(r io.Reader, p any) error {
	return gob.NewDecoder(r).Decode(p)
}
