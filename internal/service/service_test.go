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

	"github.com/Decentr-net/vulcan/internal/blockchain"
	"github.com/Decentr-net/vulcan/internal/mail"
	"github.com/Decentr-net/vulcan/internal/storage"
)

var (
	errTest     = fmt.Errorf("test")
	testOwner   = "9790d13a4778f68308977117dd470bb4"
	testAddress = "decentr1vg085ra5hw8mx5rrheqf8fruks0xv4urqkuqga"
	testEmail   = "decentr@decentr.xyz"
	testCode    = "1234"

	testInitialStakes = int64(10)
)

func TestService_Register(t *testing.T) {
	tt := []struct {
		name      string
		req       *storage.Request
		getErr    error
		setErr    error
		senderErr error
		err       error
	}{
		{
			name:   "success",
			getErr: storage.ErrNotFound,
		},
		{
			name: "already registered",
			req:  &storage.Request{ConfirmedAt: pq.NullTime{Valid: true}},
			err:  ErrAlreadyExists,
		},
		{
			name: "too many attempts",
			req:  &storage.Request{CreatedAt: time.Now()},
			err:  ErrTooManyAttempts,
		},
		{
			name: "not confirmed request already exists",
			req:  &storage.Request{Owner: testOwner, Email: testEmail, Address: testAddress, Code: testCode},
		},
		{
			name:   "getFailed",
			getErr: errTest,
			err:    errTest,
		},
		{
			name:   "setFailed",
			getErr: storage.ErrNotFound,
			setErr: errTest,
			err:    errTest,
		},
		{
			name:      "senderFailed",
			getErr:    storage.ErrNotFound,
			senderErr: errTest,
			err:       errTest,
		},
	}

	for i := range tt {
		tc := tt[i]
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)

			st := storage.NewMockStorage(ctrl)
			sender := mail.NewMockSender(ctrl)

			ctx := context.Background()

			s := New(st, sender, nil, testInitialStakes)

			var code string
			st.EXPECT().GetRequest(ctx, testOwner, testAddress).Return(tc.req, tc.getErr)
			if tc.getErr == nil || tc.getErr == storage.ErrNotFound {
				st.EXPECT().SetRequest(ctx, gomock.Any()).DoAndReturn(func(_ context.Context, r *storage.Request) error {
					assert.False(t, r.CreatedAt.IsZero())

					assert.Equal(t, testOwner, r.Owner)
					assert.Equal(t, testEmail, r.Email)
					assert.Equal(t, testAddress, r.Address)

					if tc.getErr == storage.ErrNotFound {
						code = r.Code
					} else {
						code = tc.req.Code
					}

					return tc.setErr
				})

				if tc.setErr == nil {
					sender.EXPECT().SendVerificationEmail(ctx, testEmail, gomock.Any()).DoAndReturn(func(_ context.Context, _, c string) error {
						assert.Equal(t, code, c)
						return tc.senderErr
					})
				}
			}

			assert.True(t, errors.Is(s.Register(ctx, testEmail, testAddress), tc.err))
		})
	}
}

func TestService_Confirm(t *testing.T) {
	tt := []struct {
		name    string
		req     storage.Request
		getErr  error
		setErr  error
		sendErr error
		err     error
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
			name:    "send error",
			req:     storage.Request{Owner: testOwner, Address: testAddress, Code: testCode},
			sendErr: errTest,
			err:     errTest,
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

			st := storage.NewMockStorage(ctrl)
			sn := mail.NewMockSender(ctrl)
			bc := blockchain.NewMockBlockchain(ctrl)

			ctx := context.Background()

			s := New(st, sn, bc, testInitialStakes)

			st.EXPECT().GetRequest(ctx, testOwner, "").Return(&tc.req, tc.getErr)

			if tc.getErr == nil {
				bc.EXPECT().SendStakes(tc.req.Address, testInitialStakes).Return(tc.sendErr)

				if tc.sendErr == nil {
					sn.EXPECT().SendWelcomeEmailAsync(ctx, tc.req.Email)

					st.EXPECT().SetRequest(ctx, gomock.Any()).DoAndReturn(func(_ context.Context, r *storage.Request) error {
						assert.Equal(t, tc.req.Owner, r.Owner)
						assert.Equal(t, tc.req.Address, r.Address)
						assert.True(t, r.ConfirmedAt.Valid)
						assert.False(t, r.ConfirmedAt.Time.IsZero())

						return tc.setErr
					})
				}
			}

			err := s.Confirm(ctx, testEmail, testCode)

			assert.True(t, errors.Is(err, tc.err), fmt.Sprintf("wanted %s got %s", tc.err, err))
		})
	}
}

func Test_getEmailHash(t *testing.T) {
	assert.Equal(t, testOwner, getEmailHash(testEmail))
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
