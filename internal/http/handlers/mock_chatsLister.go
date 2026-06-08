// Code generated manually to match mockery style. DO NOT EDIT.

package handlers

import (
	context "context"

	domain "github.com/ravilock/sentir-mais-backend/internal/domain"
	mock "github.com/stretchr/testify/mock"
)

type mockChatsLister struct {
	mock.Mock
}

type mockChatsLister_Expecter struct {
	mock *mock.Mock
}

func (_m *mockChatsLister) EXPECT() *mockChatsLister_Expecter {
	return &mockChatsLister_Expecter{mock: &_m.Mock}
}

func (_m *mockChatsLister) ListChats(ctx context.Context, userID string) ([]domain.ChatSummary, error) {
	ret := _m.Called(ctx, userID)

	if len(ret) == 0 {
		panic("no return value specified for ListChats")
	}

	var r0 []domain.ChatSummary
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string) ([]domain.ChatSummary, error)); ok {
		return rf(ctx, userID)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string) []domain.ChatSummary); ok {
		r0 = rf(ctx, userID)
	} else if ret.Get(0) != nil {
		r0 = ret.Get(0).([]domain.ChatSummary)
	}

	if rf, ok := ret.Get(1).(func(context.Context, string) error); ok {
		r1 = rf(ctx, userID)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

type mockChatsLister_ListChats_Call struct {
	*mock.Call
}

func (_e *mockChatsLister_Expecter) ListChats(ctx interface{}, userID interface{}) *mockChatsLister_ListChats_Call {
	return &mockChatsLister_ListChats_Call{Call: _e.mock.On("ListChats", ctx, userID)}
}

func (_c *mockChatsLister_ListChats_Call) Run(run func(ctx context.Context, userID string)) *mockChatsLister_ListChats_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(string))
	})
	return _c
}

func (_c *mockChatsLister_ListChats_Call) Return(_a0 []domain.ChatSummary, _a1 error) *mockChatsLister_ListChats_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *mockChatsLister_ListChats_Call) RunAndReturn(run func(context.Context, string) ([]domain.ChatSummary, error)) *mockChatsLister_ListChats_Call {
	_c.Call.Return(run)
	return _c
}

func newMockChatsLister(t interface {
	mock.TestingT
	Cleanup(func())
}) *mockChatsLister {
	mock := &mockChatsLister{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
