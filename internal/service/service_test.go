package service

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/lib/pq"
	"github.com/stretchr/testify/assert"

	blockchainmock "github.com/Decentr-net/vulcan/internal/blockchain/mock"
	mailmock "github.com/Decentr-net/vulcan/internal/mail/mock"
	"github.com/Decentr-net/vulcan/internal/storage"
	storagemock "github.com/Decentr-net/vulcan/internal/storage/mock"
)

var (
	errTest     = fmt.Errorf("test")
	testOwner   = "9790d13a4778f68308977117dd470bb4"
	testAddress = "decentr1vg085ra5hw8mx5rrheqf8fruks0xv4urqkuqga"
	testEmail   = "decentr@decentr.xyz"
	testCode    = "1234"

	testInitialStakes = int64(10)
	mainInitialStakes = int64(100)
)

func TestService_Register(t *testing.T) {
	tt := []struct {
		name            string
		req             *storage.Request
		getByAddressErr error
		getByOwnerErr   error
		setErr          error
		senderErr       error
		err             error
	}{
		{
			name:            "success",
			getByAddressErr: storage.ErrNotFound,
			getByOwnerErr:   storage.ErrNotFound,
		},
		{
			name: "already registered",
			req:  &storage.Request{Owner: testOwner, ConfirmedAt: pq.NullTime{Valid: true}},
			err:  ErrAlreadyExists,
		},
		{
			name: "too many attempts",
			req:  &storage.Request{Owner: getEmailHash(testEmail), Email: testEmail, CreatedAt: time.Now()},
			err:  ErrTooManyAttempts,
		},
		{
			name: "not confirmed request already exists",
			req:  &storage.Request{Owner: getEmailHash(testEmail), Email: testEmail, Address: testAddress, Code: testCode},
		},
		{
			name:            "getFailed",
			getByAddressErr: errTest,
			err:             errTest,
		},
		{
			name:            "getFailed",
			getByAddressErr: storage.ErrNotFound,
			getByOwnerErr:   errTest,
			err:             errTest,
		},
		{
			name:            "errAddressIsBusy",
			getByAddressErr: storage.ErrNotFound,
			getByOwnerErr:   storage.ErrNotFound,
			setErr:          storage.ErrAddressIsTaken,
			err:             ErrAlreadyExists,
		},
		{
			name:            "setFailed",
			getByAddressErr: storage.ErrNotFound,
			getByOwnerErr:   storage.ErrNotFound,
			setErr:          errTest,
			err:             errTest,
		},
		{
			name:            "senderFailed",
			getByAddressErr: storage.ErrNotFound,
			getByOwnerErr:   storage.ErrNotFound,
			senderErr:       errTest,
			err:             errTest,
		},
	}

	for i := range tt {
		tc := tt[i]
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)

			st := storagemock.NewMockStorage(ctrl)
			sender := mailmock.NewMockSender(ctrl)

			ctx := context.Background()

			s := New(st, sender, nil, nil, testInitialStakes, mainInitialStakes)

			var code string
			st.EXPECT().GetRequestByAddress(ctx, testAddress).Return(tc.req, tc.getByAddressErr)
			if tc.getByAddressErr == storage.ErrNotFound {
				st.EXPECT().GetRequestByOwner(ctx, testOwner).Return(tc.req, tc.getByOwnerErr)
			}

			if (tc.getByAddressErr == nil || tc.getByAddressErr == storage.ErrNotFound) &&
				(tc.getByOwnerErr == nil || tc.getByOwnerErr == storage.ErrNotFound) {
				sender.EXPECT().SendVerificationEmail(ctx, testEmail, gomock.Any()).DoAndReturn(func(_ context.Context, _, c string) error {
					code = c
					return tc.senderErr
				})

				if tc.senderErr == nil {
					st.EXPECT().UpsertRequest(ctx, testOwner, testEmail, testAddress, gomock.Not(gomock.Len(0))).DoAndReturn(
						func(_ context.Context, _, _, _, c string) error {
							assert.Equal(t, code, c)
							return tc.setErr
						})
				}
			}

			assert.True(t, errors.Is(s.Register(ctx, testEmail, testAddress), tc.err))
		})
	}
}

func TestService_Confirm(t *testing.T) {
	tt := []struct {
		name        string
		req         storage.Request
		getErr      error
		setErr      error
		testSendErr error
		mainSendErr error
		err         error
	}{
		{
			name: "success",
			req:  storage.Request{Owner: testOwner, Address: testAddress, Code: testCode},
		},
		{
			name:   "not found",
			getErr: storage.ErrNotFound,
			err:    ErrNotFound,
		},
		{
			name: "wrong code",
			req:  storage.Request{Owner: testOwner, Address: testAddress, Code: "wrong"},
			err:  ErrNotFound,
		},

		{
			name:   "check error",
			getErr: errTest,
			err:    errTest,
		},
		{
			name:        "test send error",
			req:         storage.Request{Owner: testOwner, Address: testAddress, Code: testCode},
			testSendErr: errTest,
			err:         errTest,
		},
		{
			name:        "main send error",
			req:         storage.Request{Owner: testOwner, Address: testAddress, Code: testCode},
			mainSendErr: errTest,
			err:         errTest,
		},
		{
			name:   "set error",
			req:    storage.Request{Owner: testOwner, Address: testAddress, Code: testCode},
			setErr: errTest,
			err:    errTest,
		},
	}

	for i := range tt {
		tc := tt[i]
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)

			st := storagemock.NewMockStorage(ctrl)
			sn := mailmock.NewMockSender(ctrl)
			btc := blockchainmock.NewMockBlockchain(ctrl)
			bmc := blockchainmock.NewMockBlockchain(ctrl)

			ctx := context.Background()

			s := New(st, sn, btc, bmc, testInitialStakes, mainInitialStakes)

			st.EXPECT().GetRequestByOwner(ctx, testOwner).Return(&tc.req, tc.getErr)

			if tc.getErr == nil {
				btc.EXPECT().SendStakes(tc.req.Address, testInitialStakes).Return(tc.testSendErr)

				if tc.testSendErr == nil {
					bmc.EXPECT().SendStakes(tc.req.Address, mainInitialStakes).Return(tc.mainSendErr)
				}

				if tc.mainSendErr == nil {
					sn.EXPECT().SendWelcomeEmailAsync(ctx, tc.req.Email)

					st.EXPECT().SetConfirmed(ctx, tc.req.Owner).Return(tc.setErr)
				}
			}

			err := s.Confirm(ctx, testEmail, testCode)

			assert.True(t, errors.Is(err, tc.err), fmt.Sprintf("wanted %s got %s", tc.err, err))
		})
	}
}

func Test_getEmailHash(t *testing.T) {
	assert.Equal(t, testOwner, getEmailHash(testEmail))
	assert.Equal(t, getEmailHash("email@email.email"), getEmailHash("Email@email.email"))
}

func Test_randomCode(t *testing.T) {
	c := randomCode()

	assert.Len(t, c, codeBytesSize*2)
	assert.NotEqual(t, c, randomCode())
}

func Test_truncatePlusPart(t *testing.T) {
	assert.Equal(t, "email@email.com", truncatePlusPart("email+acc1@email.com"))
	assert.Equal(t, "email@email.com", truncatePlusPart("email@email.com"))
}
