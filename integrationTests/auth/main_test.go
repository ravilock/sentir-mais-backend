package auth

import (
	"log"
	"os"
	"testing"

	integrationtests "github.com/ravilock/sentir-mais-backend/integrationTests"
)

func TestMain(m *testing.M) {
	integrationtests.RequireMongoAvailable()
	if err := integrationtests.Setup(); err != nil {
		log.Fatalf("failed to set up integration tests: %v", err)
	}

	code := m.Run()

	if err := integrationtests.Teardown(); err != nil {
		log.Printf("failed to tear down integration tests: %v", err)
	}

	os.Exit(code)
}
