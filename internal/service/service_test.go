package service

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Decentr-net/vulcan/internal/blockchain"
	"github.com/Decentr-net/vulcan/internal/mail"
	"github.com/Decentr-net/vulcan/internal/storage"
)

var (
	errTest     = fmt.Errorf("test")
	testOwner   = "be0e9f2c97c4df30483a97ab305a4046"
	testAddress = "decentr1vg085ra5hw8mx5rrheqf8fruks0xv4urqkuqga"
	testEmail   = "decentr@decentr.xyz"
	testCode    = "1234"

	testInitialStakes = int64(10)
)

func TestService_Register(t *testing.T) {
	tt := []struct {
		name      string
		createErr error
		senderErr error
		err       error
	}{
		{
			name: "success",
		},
		{
			name:      "already registered",
			createErr: storage.ErrAlreadyExists,
			err:       ErrAlreadyExists,
		},
		{
			name:      "createFailed",
			createErr: errTest,
			err:       errTest,
		},
		{
			name:      "senderFailed",
			senderErr: errTest,
			err:       errTest,
		},
	}

	for i := range tt {
		tc := tt[i]
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)

			storage := storage.NewMockStorage(ctrl)
			sender := mail.NewMockSender(ctrl)

			ctx := context.Background()

			s := New(storage, sender, nil, testInitialStakes)

			var code string

			storage.EXPECT().CreateRequest(ctx, testOwner, testAddress, gomock.Any()).DoAndReturn(func(_ context.Context, _, _, c string) error {
				require.NotEmpty(t, c)

				code = c
				return tc.createErr
			})
			if tc.createErr == nil {
				sender.EXPECT().Send(ctx, testEmail, testOwner, gomock.Any()).DoAndReturn(func(_ context.Context, _, _, c string) error {
					require.Equal(t, code, c)

					return tc.senderErr
				})
			}

			assert.True(t, errors.Is(s.Register(ctx, testEmail, testAddress), tc.err))
		})
	}
}

func TestService_Confirm(t *testing.T) {
	tt := []struct {
		name       string
		address    string
		getAddrErr error
		createErr  error
		sendErr    error
		markErr    error
		err        error
	}{
		{
			name:    "success",
			address: testAddress,
		},
		{
			name:       "not found",
			getAddrErr: storage.ErrNotFound,
			err:        ErrNotFound,
		},
		{
			name:       "check error",
			getAddrErr: errTest,
			err:        errTest,
		},
		{
			name:    "send error",
			sendErr: errTest,
			err:     errTest,
		},
		{
			name:    "mark error",
			markErr: errTest,
			err:     errTest,
		},
	}

	for i := range tt {
		tc := tt[i]
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)

			storage := storage.NewMockStorage(ctrl)
			bc := blockchain.NewMockBlockchain(ctrl)

			ctx := context.Background()

			s := New(storage, nil, bc, testInitialStakes)

			storage.EXPECT().GetNotConfirmedAccountAddress(ctx, testOwner, testCode).Return(tc.address, tc.getAddrErr)

			if tc.getAddrErr == nil {
				if tc.createErr == nil {
					bc.EXPECT().SendStakes(ctx, tc.address, testInitialStakes).Return(tc.sendErr)

					if tc.sendErr == nil {
						storage.EXPECT().MarkConfirmed(ctx, testOwner).Return(tc.markErr)
					}
				}
			}

			assert.True(t, errors.Is(s.Confirm(ctx, testOwner, testCode), tc.err))
		})
	}
}

func Test_getEmailHash(t *testing.T) {
	assert.Equal(t, testOwner, getEmailHash(testEmail))
}

func Test_randomCode(t *testing.T) {
	c := randomCode()

	assert.Len(t, c, codeSize*2)
	assert.NotEqual(t, c, randomCode())
}
