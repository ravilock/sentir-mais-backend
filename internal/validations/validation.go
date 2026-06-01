package validations

import (
	"sync"

	"github.com/go-playground/validator/v10"
	"github.com/go-playground/validator/v10/non-standard/validators"
)

var (
	Validate *validator.Validate
	once     sync.Once
	initErr  error
)

func InitValidator() error {
	once.Do(func() {
		Validate = validator.New()
		initErr = Validate.RegisterValidation("notblank", validators.NotBlank)
	})

	return initErr
}
