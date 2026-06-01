// Code generated manually to match mockery style. DO NOT EDIT.

package services

import (
	context "context"

	domain "github.com/ravilock/sentir-mais-backend/internal/domain"
	mock "github.com/stretchr/testify/mock"
)

type mockFeelingClassifier struct {
	mock.Mock
}

type mockFeelingClassifier_Expecter struct {
	mock *mock.Mock
}

func (_m *mockFeelingClassifier) EXPECT() *mockFeelingClassifier_Expecter {
	return &mockFeelingClassifier_Expecter{mock: &_m.Mock}
}

func (_m *mockFeelingClassifier) Classify(ctx context.Context, text string) (domain.ClassificationResult, error) {
	ret := _m.Called(ctx, text)

	if len(ret) == 0 {
		panic("no return value specified for Classify")
	}

	var r0 domain.ClassificationResult
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string) (domain.ClassificationResult, error)); ok {
		return rf(ctx, text)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string) domain.ClassificationResult); ok {
		r0 = rf(ctx, text)
	} else {
		r0 = ret.Get(0).(domain.ClassificationResult)
	}

	if rf, ok := ret.Get(1).(func(context.Context, string) error); ok {
		r1 = rf(ctx, text)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

type mockFeelingClassifier_Classify_Call struct {
	*mock.Call
}

func (_e *mockFeelingClassifier_Expecter) Classify(ctx interface{}, text interface{}) *mockFeelingClassifier_Classify_Call {
	return &mockFeelingClassifier_Classify_Call{Call: _e.mock.On("Classify", ctx, text)}
}

func (_c *mockFeelingClassifier_Classify_Call) Run(run func(ctx context.Context, text string)) *mockFeelingClassifier_Classify_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(string))
	})
	return _c
}

func (_c *mockFeelingClassifier_Classify_Call) Return(_a0 domain.ClassificationResult, _a1 error) *mockFeelingClassifier_Classify_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *mockFeelingClassifier_Classify_Call) RunAndReturn(run func(context.Context, string) (domain.ClassificationResult, error)) *mockFeelingClassifier_Classify_Call {
	_c.Call.Return(run)
	return _c
}

func newMockFeelingClassifier(t interface {
	mock.TestingT
	Cleanup(func())
}) *mockFeelingClassifier {
	mock := &mockFeelingClassifier{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
