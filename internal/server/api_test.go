package server

import (
	"testing"

	"github.com/stretchr/testify/require"
)

const testAddress = "decentr18c2phdrfjkggr4afwf3rw4h4xsjvfhh2gl7t4m"

func TestRegisterRequest_Validate(t *testing.T) {
	tt := []struct {
		name  string
		req   RegisterRequest
		valid bool
	}{
		{
			name: "invalid_email_1",
			req: RegisterRequest{
				Email:   "111@mailru",
				Address: testAddress,
			},
			valid: false,
		},
		{
			name: "valid_email_1",
			req: RegisterRequest{
				Email:   "111@mail.ru",
				Address: testAddress,
			},
			valid: true,
		},
		{
			name: "valid_email_2",
			req: RegisterRequest{
				Email:   "111+111@mail.ru",
				Address: testAddress,
			},
			valid: true,
		},
		{
			name: "invalid_address_1",
			req: RegisterRequest{
				Email:   "111+111@mail.ru",
				Address: "18c2phdrfjkggr4afwf3rw4h4xsjvfhh2gl7t4m",
			},
			valid: false,
		},
		{
			name: "invalid_address_1",
			req: RegisterRequest{
				Email:   "111+111@mail.ru",
				Address: "decentr18c2phdrfjkggr4afwf3rw4h4xsjvfhh2g",
			},
			valid: false,
		},
	}

	for i := range tt {
		tc := tt[i]
		t.Run(tc.name, func(t *testing.T) {
			if tc.valid {
				require.NoError(t, tc.req.validate())
			} else {
				require.Error(t, tc.req.validate())
			}
		})
	}
}
