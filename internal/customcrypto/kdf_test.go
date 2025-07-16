package customcrypto

import (
	"log"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDeriveKey(t *testing.T) {
	name := "output"
	pwd := "12345678900"
	salt := []byte{25, 177, 201, 141, 182, 62, 208, 116, 116, 226, 5, 241, 53, 254, 69, 178, 121, 251, 217, 152, 130, 206, 236, 99, 147, 109, 247, 6, 93, 17, 109, 253}

	t.Run(name, func(t *testing.T) {
		gotDerivedKey, gotUsedSalt, err := deriveKey(
			[]byte(pwd),
			salt,
		)
		assert.Nil(t, err)
		assert.Equal(t, salt[0], gotUsedSalt[0])

		nextGotDerivedKey, nextGotUsedSalt, err := deriveKey(
			[]byte(pwd),
			gotUsedSalt,
		)
		assert.Nil(t, err)
		assert.Equal(t, gotUsedSalt[0], nextGotUsedSalt[0])
		assert.Equal(t, gotDerivedKey[0], nextGotDerivedKey[0])

		log.Println(gotDerivedKey[0])
		log.Println(nextGotDerivedKey[0])
	})
}
