/*
	Copyright (c) 2025 Stephen Jersuit Benyah
	Licensed under the Repo-Only Non-Commercial & No-Derivatives License with Anti-Training Clause (RONCND-AT) v1.0.
	See LICENSE and CONTRIBUTION_LICENSE_AGREEMENT.md in repository root.
	Prohibited: copying, reuse, redistribution, or use as training data for machine learning/AI.
*/

package customcrypto

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCustomCipher_Encrypt(t *testing.T) {
	type args struct {
		data      string
		masterKey string
	}
	tests := []struct {
		name           string
		c              cCipher
		args           args
		wantCipherData []byte
		wantNonce      []byte
		wantErr        bool
	}{
		{
			name: "test1",
			args: args{
				data:      "test",
				masterKey: "12345678901234567890123456789012",
			},
		},
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := cCipher{}
			gotCipherData, gotNonce, err := c.Encrypt(
				[]byte(tt.args.data),
				[]byte(tt.args.masterKey),
			)
			assert.Nil(t, err)

			cData, err := c.Decrypt(
				[]byte(tt.args.masterKey),
				gotNonce,
				gotCipherData,
			)
			assert.Nil(t, err)

			assert.Equal(t, []byte(tt.args.data), cData)
		})
	}
}
