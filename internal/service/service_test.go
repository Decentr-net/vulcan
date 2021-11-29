package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Decentr-net/vulcan/internal/blockchain"

	"github.com/golang/mock/gomock"
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
			req:  &storage.Request{Owner: testOwner, ConfirmedAt: sql.NullTime{Valid: true}},
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

			s := &service{
				storage:           st,
				sender:            sender,
				initialTestStakes: testInitialStakes,
				initialMainStakes: mainInitialStakes,
			}

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
					st.EXPECT().UpsertRequest(ctx, testOwner, testEmail, testAddress, gomock.Not(gomock.Len(0)), sql.NullString{}).DoAndReturn(
						func(_ context.Context, _, _, _, c string, _ sql.NullString) error {
							assert.Equal(t, code, c)
							return tc.setErr
						})
				}
			}

			assert.True(t, errors.Is(s.Register(ctx, testEmail, testAddress, nil), tc.err))
		})
	}
}

func TestService_GetReferralCode(t *testing.T) {
	tt := []struct {
		name   string
		req    storage.Request
		getErr error
		err    error
	}{
		{
			name: "success",
			req:  storage.Request{Owner: testOwner, Address: testAddress, OwnReferralCode: testCode},
		},
		{
			name:   "not found",
			getErr: storage.ErrNotFound,
			err:    ErrRequestNotFound,
		},
	}

	for i := range tt {
		tc := tt[i]
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			ctx := context.Background()
			st := storagemock.NewMockStorage(ctrl)

			s := &service{
				storage:           st,
				initialTestStakes: testInitialStakes,
				initialMainStakes: mainInitialStakes,
			}

			st.EXPECT().GetRequestByAddress(ctx, testAddress).Return(&tc.req, tc.getErr)
			code, err := s.GetOwnReferralCode(ctx, testAddress)
			if err != nil {
				assert.True(t, errors.Is(err, tc.err), fmt.Sprintf("wanted %s got %s", tc.err, err))
			}
			assert.Equal(t, tc.req.OwnReferralCode, code)
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
			err:    ErrRequestNotFound,
		},
		{
			name: "wrong code",
			req:  storage.Request{Owner: testOwner, Address: testAddress, Code: "wrong"},
			err:  ErrRequestNotFound,
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

			s := &service{
				storage:           st,
				sender:            sn,
				btc:               btc,
				bmc:               bmc,
				initialTestStakes: testInitialStakes,
				initialMainStakes: mainInitialStakes,
			}

			st.EXPECT().GetRequestByOwner(ctx, testOwner).Return(&tc.req, tc.getErr)

			if tc.getErr == nil {
				btc.EXPECT().SendStakes([]blockchain.Stake{
					{Address: tc.req.Address, Amount: testInitialStakes},
				}, "").Return(tc.testSendErr)
				bmc.EXPECT().SendStakes([]blockchain.Stake{
					{Address: tc.req.Address, Amount: mainInitialStakes},
				}, "").Return(tc.mainSendErr)

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

func Test_transformStatsAsGrowth(t *testing.T) {
	const (
		day   = 24 * time.Hour
		total = 10
	)

	date := time.Now()

	stats := []*storage.RegisterStats{
		{Date: date.Add(-3 * day), Value: 1},
		{Date: date.Add(-1 * day), Value: 3},
		{Date: date.Add(-2 * day), Value: 2},
		{Date: date, Value: 4},
	}

	transformStatsAsGrowth(stats, total)

	//1 2 3 4 => 1 3 6 10
	require.Equal(t, storage.RegisterStats{Date: date.Add(-3 * day), Value: 1}, *stats[0])
	require.Equal(t, storage.RegisterStats{Date: date.Add(-2 * day), Value: 3}, *stats[1])
	require.Equal(t, storage.RegisterStats{Date: date.Add(-1 * day), Value: 6}, *stats[2])
	require.Equal(t, storage.RegisterStats{Date: date, Value: total}, *stats[3])
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
