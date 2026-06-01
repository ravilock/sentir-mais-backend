// Code generated manually to match mockery style. DO NOT EDIT.

package services

import (
	context "context"

	domain "github.com/ravilock/sentir-mais-backend/internal/domain"
	mock "github.com/stretchr/testify/mock"
)

type mockMessageAnalysisCreator struct {
	mock.Mock
}

type mockMessageAnalysisCreator_Expecter struct {
	mock *mock.Mock
}

func (_m *mockMessageAnalysisCreator) EXPECT() *mockMessageAnalysisCreator_Expecter {
	return &mockMessageAnalysisCreator_Expecter{mock: &_m.Mock}
}

func (_m *mockMessageAnalysisCreator) Create(ctx context.Context, analysis domain.MessageAnalysis) error {
	ret := _m.Called(ctx, analysis)

	if len(ret) == 0 {
		panic("no return value specified for Create")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, domain.MessageAnalysis) error); ok {
		r0 = rf(ctx, analysis)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

type mockMessageAnalysisCreator_Create_Call struct {
	*mock.Call
}

func (_e *mockMessageAnalysisCreator_Expecter) Create(ctx interface{}, analysis interface{}) *mockMessageAnalysisCreator_Create_Call {
	return &mockMessageAnalysisCreator_Create_Call{Call: _e.mock.On("Create", ctx, analysis)}
}

func (_c *mockMessageAnalysisCreator_Create_Call) Run(run func(ctx context.Context, analysis domain.MessageAnalysis)) *mockMessageAnalysisCreator_Create_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(domain.MessageAnalysis))
	})
	return _c
}

func (_c *mockMessageAnalysisCreator_Create_Call) Return(_a0 error) *mockMessageAnalysisCreator_Create_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *mockMessageAnalysisCreator_Create_Call) RunAndReturn(run func(context.Context, domain.MessageAnalysis) error) *mockMessageAnalysisCreator_Create_Call {
	_c.Call.Return(run)
	return _c
}

func newMockMessageAnalysisCreator(t interface {
	mock.TestingT
	Cleanup(func())
}) *mockMessageAnalysisCreator {
	mock := &mockMessageAnalysisCreator{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
