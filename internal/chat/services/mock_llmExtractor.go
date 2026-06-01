// Code generated manually to match mockery style. DO NOT EDIT.

package services

import (
	context "context"

	domain "github.com/ravilock/sentir-mais-backend/internal/domain"
	mock "github.com/stretchr/testify/mock"
)

type mockLlmExtractor struct {
	mock.Mock
}

type mockLlmExtractor_Expecter struct {
	mock *mock.Mock
}

func (_m *mockLlmExtractor) EXPECT() *mockLlmExtractor_Expecter {
	return &mockLlmExtractor_Expecter{mock: &_m.Mock}
}

func (_m *mockLlmExtractor) ExtractEvent(ctx context.Context, history []domain.Message) (domain.ExtractedEvent, error) {
	ret := _m.Called(ctx, history)

	if len(ret) == 0 {
		panic("no return value specified for ExtractEvent")
	}

	var r0 domain.ExtractedEvent
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, []domain.Message) (domain.ExtractedEvent, error)); ok {
		return rf(ctx, history)
	}
	if rf, ok := ret.Get(0).(func(context.Context, []domain.Message) domain.ExtractedEvent); ok {
		r0 = rf(ctx, history)
	} else {
		r0 = ret.Get(0).(domain.ExtractedEvent)
	}

	if rf, ok := ret.Get(1).(func(context.Context, []domain.Message) error); ok {
		r1 = rf(ctx, history)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

type mockLlmExtractor_ExtractEvent_Call struct {
	*mock.Call
}

func (_e *mockLlmExtractor_Expecter) ExtractEvent(ctx interface{}, history interface{}) *mockLlmExtractor_ExtractEvent_Call {
	return &mockLlmExtractor_ExtractEvent_Call{Call: _e.mock.On("ExtractEvent", ctx, history)}
}

func (_c *mockLlmExtractor_ExtractEvent_Call) Run(run func(ctx context.Context, history []domain.Message)) *mockLlmExtractor_ExtractEvent_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].([]domain.Message))
	})
	return _c
}

func (_c *mockLlmExtractor_ExtractEvent_Call) Return(_a0 domain.ExtractedEvent, _a1 error) *mockLlmExtractor_ExtractEvent_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *mockLlmExtractor_ExtractEvent_Call) RunAndReturn(run func(context.Context, []domain.Message) (domain.ExtractedEvent, error)) *mockLlmExtractor_ExtractEvent_Call {
	_c.Call.Return(run)
	return _c
}

func newMockLlmExtractor(t interface {
	mock.TestingT
	Cleanup(func())
}) *mockLlmExtractor {
	mock := &mockLlmExtractor{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
