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
	errTest   = fmt.Errorf("test")
	testOwner = "be0e9f2c97c4df30483a97ab305a4046"
	testEmail = "decentr@decentr.xyz"
	testCode  = "1234"

	testInitialStakes = int64(10)
)

func TestService_Register(t *testing.T) {
	tt := []struct {
		name            string
		isRegistered    bool
		isRegisteredErr error
		createErr       error
		senderErr       error
		err             error
	}{
		{
			name:         "success",
			isRegistered: false,
		},
		{
			name:         "already registered",
			isRegistered: true,
			err:          ErrEmailIsBusy,
		},
		{
			name:            "isRegisteredFailed",
			isRegistered:    false,
			isRegisteredErr: errTest,
			err:             errTest,
		},
		{
			name:         "createFailed",
			isRegistered: false,
			createErr:    errTest,
			err:          errTest,
		},
		{
			name:         "senderFailed",
			isRegistered: false,
			senderErr:    errTest,
			err:          errTest,
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

			storage.EXPECT().IsRegistered(ctx, testOwner).Return(tc.isRegistered, tc.isRegisteredErr)
			if tc.isRegisteredErr == nil && !tc.isRegistered {
				var code string

				storage.EXPECT().CreateRequest(ctx, testOwner, gomock.Any()).DoAndReturn(func(_ context.Context, _, c string) error {
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
			}

			assert.True(t, errors.Is(s.Register(ctx, testEmail), tc.err))
		})
	}
}

func TestService_Confirm(t *testing.T) {
	tt := []struct {
		name      string
		checkErr  error
		createErr error
		sendErr   error
		markErr   error
		err       error
		res       AccountInfo
	}{
		{
			name: "success",
			res: AccountInfo{
				Address:  "1234",
				PubKey:   "5678",
				Mnemonic: []string{"1", "2"},
			},
		},
		{
			name:     "not found",
			checkErr: storage.ErrNotFound,
			err:      ErrNotFound,
		},
		{
			name:     "check error",
			checkErr: errTest,
			err:      errTest,
		},
		{
			name:      "create error",
			createErr: errTest,
			err:       errTest,
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

			storage.EXPECT().CheckRequest(ctx, testOwner, testCode).Return(tc.checkErr)

			if tc.checkErr == nil {
				bc.EXPECT().CreateWallet(ctx).DoAndReturn(func(_ context.Context) (blockchain.AccountInfo, error) {
					if tc.createErr != nil {
						return blockchain.AccountInfo{}, tc.createErr
					}

					return blockchain.AccountInfo{
						Mnemonic: tc.res.Mnemonic,
						PubKey:   tc.res.PubKey,
						Address:  tc.res.Address,
					}, nil
				})

				if tc.createErr == nil {
					bc.EXPECT().SendStakes(ctx, tc.res.Address, testInitialStakes).Return(tc.sendErr)

					if tc.sendErr == nil {
						storage.EXPECT().MarkRequestProcessed(ctx, testOwner).Return(tc.markErr)
					}
				}
			}

			acc, err := s.Confirm(ctx, testOwner, testCode)
			assert.True(t, errors.Is(err, tc.err))
			assert.Equal(t, tc.res, acc)
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
