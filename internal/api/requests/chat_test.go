package requests

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCreateChatRequestValidate(t *testing.T) {
	t.Run("valid request should not return errors", func(t *testing.T) {
		request := &CreateChatRequest{InitialMessage: "hello"}
		require.NoError(t, request.Validate())
	})

	t.Run("message should not be blank", func(t *testing.T) {
		request := &CreateChatRequest{InitialMessage: " "}
		err := request.Validate()
		require.ErrorContains(t, err, "notblank")
		require.ErrorContains(t, err, "InitialMessage")
	})
}

func TestSendMessageRequestValidate(t *testing.T) {
	t.Run("message is required", func(t *testing.T) {
		request := &SendMessageRequest{}
		err := request.Validate()
		require.ErrorContains(t, err, "required")
		require.ErrorContains(t, err, "Message")
	})
}
