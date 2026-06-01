package requests

import (
	"log"
	"os"
	"testing"

	"github.com/ravilock/sentir-mais-backend/internal/validations"
)

func TestMain(m *testing.M) {
	if err := validations.InitValidator(); err != nil {
		log.Fatalln("Failed to load validator", err)
	}

	code := m.Run()
	os.Exit(code)
}
