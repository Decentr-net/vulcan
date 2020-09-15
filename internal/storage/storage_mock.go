// Code generated by MockGen. DO NOT EDIT.
// Source: storage.go

// Package storage is a generated GoMock package.
package storage

import (
	context "context"
	gomock "github.com/golang/mock/gomock"
	reflect "reflect"
)

// MockStorage is a mock of Storage interface
type MockStorage struct {
	ctrl     *gomock.Controller
	recorder *MockStorageMockRecorder
}

// MockStorageMockRecorder is the mock recorder for MockStorage
type MockStorageMockRecorder struct {
	mock *MockStorage
}

// NewMockStorage creates a new mock instance
func NewMockStorage(ctrl *gomock.Controller) *MockStorage {
	mock := &MockStorage{ctrl: ctrl}
	mock.recorder = &MockStorageMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use
func (m *MockStorage) EXPECT() *MockStorageMockRecorder {
	return m.recorder
}

// CreateRequest mocks base method
func (m *MockStorage) CreateRequest(ctx context.Context, owner, address, code string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateRequest", ctx, owner, address, code)
	ret0, _ := ret[0].(error)
	return ret0
}

// CreateRequest indicates an expected call of CreateRequest
func (mr *MockStorageMockRecorder) CreateRequest(ctx, owner, address, code interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateRequest", reflect.TypeOf((*MockStorage)(nil).CreateRequest), ctx, owner, address, code)
}

// GetNotConfirmedAccountAddress mocks base method
func (m *MockStorage) GetNotConfirmedAccountAddress(ctx context.Context, owner, code string) (string, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetNotConfirmedAccountAddress", ctx, owner, code)
	ret0, _ := ret[0].(string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetNotConfirmedAccountAddress indicates an expected call of GetNotConfirmedAccountAddress
func (mr *MockStorageMockRecorder) GetNotConfirmedAccountAddress(ctx, owner, code interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetNotConfirmedAccountAddress", reflect.TypeOf((*MockStorage)(nil).GetNotConfirmedAccountAddress), ctx, owner, code)
}

// MarkRequestConfirmed mocks base method
func (m *MockStorage) MarkRequestConfirmed(ctx context.Context, owner string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "MarkRequestConfirmed", ctx, owner)
	ret0, _ := ret[0].(error)
	return ret0
}

// MarkRequestConfirmed indicates an expected call of MarkRequestConfirmed
func (mr *MockStorageMockRecorder) MarkRequestConfirmed(ctx, owner interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "MarkRequestConfirmed", reflect.TypeOf((*MockStorage)(nil).MarkRequestConfirmed), ctx, owner)
}
