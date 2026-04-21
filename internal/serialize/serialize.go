package serialize

import (
	"encoding/gob"
	"io"
)

type Serializer interface {
	Encode(io.Writer, any) error
	Decode(r io.Reader, p any) error
	Register(types ...any)
}

type serialize struct{}

func New() *serialize {
	return &serialize{}
}

func (serialize) Encode(w io.Writer, p any) error {
	return gob.NewEncoder(w).Encode(p)
}

func (serialize) Decode(r io.Reader, p any) error {
	return gob.NewDecoder(r).Decode(p)
}

/*
Register registers all types that would be encoded and decoded as any/interface
types. You can register both the pointer type or value type.
eg. Register(&type, type)
*/
func (serialize) Register(types ...any) {
	for _, v := range types {
		// Use a closure to handle each registration separately
		func(val any) {
			defer func() {
				if r := recover(); r != nil {
					// Type was already registered, just continue
				}
			}()
			gob.Register(val)
			// log.Println(v)
		}(v)
	}
}
