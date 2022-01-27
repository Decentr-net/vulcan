package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"testing"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	"github.com/Decentr-net/vulcan/internal/blockchain"
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

	initialStakes = sdk.NewInt(100)
)

func TestService_Register(t *testing.T) {
	tt := []struct {
		name          string
		mockSetupFunc func(s *storagemock.MockStorage, m *mailmock.MockSender)
		err           error
	}{
		{
			name: "success",
			mockSetupFunc: func(s *storagemock.MockStorage, m *mailmock.MockSender) {
				s.EXPECT().GetRequestByAddress(gomock.Any(), testAddress).Return(nil, storage.ErrNotFound)
				s.EXPECT().GetRequestByOwner(gomock.Any(), testOwner).Return(nil, storage.ErrNotFound)
				var code string
				s.EXPECT().UpsertRequest(gomock.Any(), testOwner, testEmail, testAddress, gomock.Not(gomock.Len(0)), sql.NullString{}).DoAndReturn(
					func(_ context.Context, _, _, _, c string, _ sql.NullString) error {
						code = c
						return nil
					},
				)
				m.EXPECT().SendVerificationEmailAsync(gomock.Any(), testEmail, gomock.Any()).Do(func(_ context.Context, _, c string) {
					assert.Equal(t, code, c)
				})
			},
		},
		{
			name: "already registered",
			mockSetupFunc: func(s *storagemock.MockStorage, m *mailmock.MockSender) {
				s.EXPECT().GetRequestByAddress(gomock.Any(), testAddress).Return(&storage.Request{Owner: testOwner, ConfirmedAt: sql.NullTime{Valid: true}}, nil)
			},
			err: ErrAlreadyExists,
		},
		{
			name: "already registered#2",
			mockSetupFunc: func(s *storagemock.MockStorage, m *mailmock.MockSender) {
				s.EXPECT().GetRequestByAddress(gomock.Any(), testAddress).Return(nil, storage.ErrNotFound)
				s.EXPECT().GetRequestByOwner(gomock.Any(), testOwner).Return(&storage.Request{Owner: testOwner, ConfirmedAt: sql.NullTime{Valid: true}}, nil)
			},
			err: ErrAlreadyExists,
		},
		{
			name: "too many attempts",
			mockSetupFunc: func(s *storagemock.MockStorage, m *mailmock.MockSender) {
				s.EXPECT().GetRequestByAddress(gomock.Any(), testAddress).Return(nil, storage.ErrNotFound)
				s.EXPECT().GetRequestByOwner(gomock.Any(), testOwner).Return(&storage.Request{Owner: getEmailHash(testEmail), Email: testEmail, CreatedAt: time.Now()}, nil)
			},
			err: ErrTooManyAttempts,
		},
		{
			name: "not confirmed request already exists",
			mockSetupFunc: func(s *storagemock.MockStorage, m *mailmock.MockSender) {
				s.EXPECT().GetRequestByAddress(gomock.Any(), testAddress).Return(nil, storage.ErrNotFound)
				s.EXPECT().GetRequestByOwner(gomock.Any(), testOwner).Return(&storage.Request{Owner: getEmailHash(testEmail), Email: testEmail, Address: testAddress, Code: testCode}, nil)
				var code string
				s.EXPECT().UpsertRequest(gomock.Any(), testOwner, testEmail, testAddress, gomock.Not(gomock.Len(0)), sql.NullString{}).DoAndReturn(
					func(_ context.Context, _, _, _, c string, _ sql.NullString) error {
						code = c
						return nil
					},
				)
				m.EXPECT().SendVerificationEmailAsync(gomock.Any(), testEmail, gomock.Any()).Do(func(_ context.Context, _, c string) {
					assert.Equal(t, code, c)
				})
			},
		},
		{
			name: "getByAddressFailed",
			mockSetupFunc: func(s *storagemock.MockStorage, m *mailmock.MockSender) {
				s.EXPECT().GetRequestByAddress(gomock.Any(), testAddress).Return(nil, errTest)
			},
			err: errTest,
		},
		{
			name: "getByOwnerFailed",
			mockSetupFunc: func(s *storagemock.MockStorage, m *mailmock.MockSender) {
				s.EXPECT().GetRequestByAddress(gomock.Any(), testAddress).Return(nil, storage.ErrNotFound)
				s.EXPECT().GetRequestByOwner(gomock.Any(), testOwner).Return(nil, errTest)
			},
			err: errTest,
		},
		{
			name: "errAddressIsBusy",
			mockSetupFunc: func(s *storagemock.MockStorage, m *mailmock.MockSender) {
				s.EXPECT().GetRequestByAddress(gomock.Any(), testAddress).Return(nil, storage.ErrNotFound)
				s.EXPECT().GetRequestByOwner(gomock.Any(), testOwner).Return(nil, storage.ErrNotFound)
				s.EXPECT().UpsertRequest(gomock.Any(), testOwner, testEmail, testAddress, gomock.Not(gomock.Len(0)), sql.NullString{}).Return(storage.ErrAddressIsTaken)
			},
			err: ErrAlreadyExists,
		},
		{
			name: "setFailed",
			mockSetupFunc: func(s *storagemock.MockStorage, m *mailmock.MockSender) {
				s.EXPECT().GetRequestByAddress(gomock.Any(), testAddress).Return(nil, storage.ErrNotFound)
				s.EXPECT().GetRequestByOwner(gomock.Any(), testOwner).Return(nil, storage.ErrNotFound)
				s.EXPECT().UpsertRequest(gomock.Any(), testOwner, testEmail, testAddress, gomock.Not(gomock.Len(0)), sql.NullString{}).Return(errTest)
			},
			err: errTest,
		},
		{
			name: "senderFailed",
			mockSetupFunc: func(s *storagemock.MockStorage, m *mailmock.MockSender) {
				s.EXPECT().GetRequestByAddress(gomock.Any(), testAddress).Return(nil, storage.ErrNotFound)
				s.EXPECT().GetRequestByOwner(gomock.Any(), testOwner).Return(nil, storage.ErrNotFound)
				s.EXPECT().UpsertRequest(gomock.Any(), testOwner, testEmail, testAddress, gomock.Not(gomock.Len(0)), sql.NullString{}).Return(nil)
				m.EXPECT().SendVerificationEmailAsync(gomock.Any(), testEmail, gomock.Any())
			},
			err: nil,
		},
	}

	for i := range tt {
		tc := tt[i]
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			st := storagemock.NewMockStorage(ctrl)
			sender := mailmock.NewMockSender(ctrl)

			ctx := context.Background()

			s := &service{
				storage:       st,
				sender:        sender,
				initialStakes: initialStakes,
			}

			tc.mockSetupFunc(st, sender)

			assert.True(t, errors.Is(s.Register(ctx, testEmail, testAddress, nil), tc.err))
			time.Sleep(100 * time.Millisecond)
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
				storage:       st,
				initialStakes: initialStakes,
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
		name          string
		mockSetupFunc func(s *storagemock.MockStorage, m *mailmock.MockSender, bc *blockchainmock.MockBlockchain)
		err           error
	}{
		{
			name: "success",
			mockSetupFunc: func(s *storagemock.MockStorage, m *mailmock.MockSender, bc *blockchainmock.MockBlockchain) {
				s.EXPECT().GetRequestByOwner(gomock.Any(), testOwner).Return(&storage.Request{
					Owner:   testOwner,
					Email:   testEmail,
					Address: testAddress,
					Code:    testCode,
				}, nil)

				bc.EXPECT().SendStakes([]blockchain.Stake{
					{Address: testAddress, Amount: initialStakes},
				}, "").Return(nil)
				m.EXPECT().SendWelcomeEmailAsync(gomock.Any(), testEmail)
				s.EXPECT().SetConfirmed(gomock.Any(), testOwner).Return(nil)
			},
		},
		{
			name: "not found",
			mockSetupFunc: func(s *storagemock.MockStorage, m *mailmock.MockSender, bc *blockchainmock.MockBlockchain) {
				s.EXPECT().GetRequestByOwner(gomock.Any(), testOwner).Return(nil, storage.ErrNotFound)
			},
			err: ErrRequestNotFound,
		},
		{
			name: "wrong code",
			mockSetupFunc: func(s *storagemock.MockStorage, m *mailmock.MockSender, bc *blockchainmock.MockBlockchain) {
				s.EXPECT().GetRequestByOwner(gomock.Any(), testOwner).Return(&storage.Request{
					Owner:   testOwner,
					Email:   testEmail,
					Address: testAddress,
					Code:    "wrong",
				}, nil)
			},
			err: ErrRequestNotFound,
		},
		{
			name: "check error",
			mockSetupFunc: func(s *storagemock.MockStorage, m *mailmock.MockSender, bc *blockchainmock.MockBlockchain) {
				s.EXPECT().GetRequestByOwner(gomock.Any(), testOwner).Return(nil, errTest)
			},
			err: errTest,
		},
		{
			name: "send error",
			mockSetupFunc: func(s *storagemock.MockStorage, m *mailmock.MockSender, bc *blockchainmock.MockBlockchain) {
				s.EXPECT().GetRequestByOwner(gomock.Any(), testOwner).Return(&storage.Request{
					Owner:   testOwner,
					Email:   testEmail,
					Address: testAddress,
					Code:    testCode,
				}, nil)
				bc.EXPECT().SendStakes([]blockchain.Stake{
					{Address: testAddress, Amount: initialStakes},
				}, "").Return(errTest)
			},
			err: errTest,
		},
		{
			name: "set error",
			mockSetupFunc: func(s *storagemock.MockStorage, m *mailmock.MockSender, bc *blockchainmock.MockBlockchain) {
				s.EXPECT().GetRequestByOwner(gomock.Any(), testOwner).Return(&storage.Request{
					Owner:   testOwner,
					Email:   testEmail,
					Address: testAddress,
					Code:    testCode,
				}, nil)
				bc.EXPECT().SendStakes([]blockchain.Stake{
					{Address: testAddress, Amount: initialStakes},
				}, "").Return(nil)
				m.EXPECT().SendWelcomeEmailAsync(gomock.Any(), testEmail)
				s.EXPECT().SetConfirmed(gomock.Any(), testOwner).Return(errTest)
			},
			err: errTest,
		},
	}

	for i := range tt {
		tc := tt[i]
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)

			st := storagemock.NewMockStorage(ctrl)
			sn := mailmock.NewMockSender(ctrl)
			bc := blockchainmock.NewMockBlockchain(ctrl)

			ctx := context.Background()

			s := &service{
				storage:       st,
				sender:        sn,
				bc:            bc,
				initialStakes: initialStakes,
			}

			tc.mockSetupFunc(st, sn, bc)

			assert.ErrorIs(t, s.Confirm(ctx, testEmail, testCode), tc.err)
		})
	}
}

func TestService_RegisterTestnetAccount(t *testing.T) {
	tt := []struct {
		name          string
		mockSetupFunc func(bc *blockchainmock.MockBlockchain, storage *storagemock.MockStorage)
		err           error
	}{
		{
			name: "success",
			mockSetupFunc: func(bc *blockchainmock.MockBlockchain, storage *storagemock.MockStorage) {
				bc.EXPECT().SendStakes([]blockchain.Stake{
					{Address: testAddress, Amount: giveStakesAmount},
				}, "").Return(nil)

				storage.EXPECT().CreateTestnetConfirmedRequest(gomock.Any(), testAddress).Return(nil)
			},
		},
		{
			name: "error",
			mockSetupFunc: func(bc *blockchainmock.MockBlockchain, storage *storagemock.MockStorage) {
				bc.EXPECT().SendStakes([]blockchain.Stake{
					{Address: testAddress, Amount: giveStakesAmount},
				}, "").Return(errTest)
			},
			err: errTest,
		},
	}

	for i := range tt {
		tc := tt[i]
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)

			bc := blockchainmock.NewMockBlockchain(ctrl)
			storage := storagemock.NewMockStorage(ctrl)

			ctx := context.Background()

			s := &service{
				bc:      bc,
				storage: storage,
			}

			tc.mockSetupFunc(bc, storage)

			assert.ErrorIs(t, s.RegisterTestnetAccount(ctx, testAddress), tc.err)
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
