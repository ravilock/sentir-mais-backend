package integrationtests

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ravilock/sentir-mais-backend/internal/api"
	apiresponses "github.com/ravilock/sentir-mais-backend/internal/api/responses"
	authrepositories "github.com/ravilock/sentir-mais-backend/internal/auth/repositories"
	"github.com/ravilock/sentir-mais-backend/internal/config"
	"github.com/ravilock/sentir-mais-backend/internal/storage/mongodb"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

const testDatabaseName = "sentir_mais_integration_tests"

var (
	testServer     *httptest.Server
	testConnection *mongodb.Connection
)

func Setup() error {
	cfg := config.Load()
	cfg.MongoDatabase = testDatabaseName
	if databaseName := os.Getenv("INTEGRATION_TEST_DATABASE"); databaseName != "" {
		cfg.MongoDatabase = databaseName
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	connection, err := mongodb.Connect(ctx, cfg.MongoURI, cfg.MongoDatabase)
	if err != nil {
		return err
	}

	if err := clearDatabase(ctx, connection); err != nil {
		_ = connection.Close(context.Background())
		return err
	}

	server, err := api.NewServer(cfg)
	if err != nil {
		_ = connection.Close(context.Background())
		return err
	}

	testConnection = connection
	testServer = httptest.NewServer(server)
	return nil
}

func Teardown() error {
	var errs []string

	if testServer != nil {
		testServer.Close()
	}

	if testConnection != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := testConnection.Database.Drop(ctx); err != nil {
			errs = append(errs, err.Error())
		}
		if err := testConnection.Close(ctx); err != nil {
			errs = append(errs, err.Error())
		}
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}

	return nil
}

func ClearDatabase(t *testing.T) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := clearDatabase(ctx, testConnection); err != nil {
		t.Fatalf("clear database: %v", err)
	}
}

func BaseURL() string {
	return testServer.URL
}

func Database() *mongo.Database {
	return testConnection.Database
}

func MustJSONRequest(t *testing.T, method, path string, body any, token string) *http.Response {
	t.Helper()

	var requestBody *bytes.Reader
	if body == nil {
		requestBody = bytes.NewReader(nil)
	} else {
		payload, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal request: %v", err)
		}
		requestBody = bytes.NewReader(payload)
	}

	req, err := http.NewRequest(method, BaseURL()+path, requestBody)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("execute request: %v", err)
	}

	return res
}

func DecodeResponse[T any](t *testing.T, res *http.Response) T {
	t.Helper()
	defer res.Body.Close()

	var value T
	if err := json.NewDecoder(res.Body).Decode(&value); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	return value
}

func MustFindUserByEmail(t *testing.T, email string) apiresponses.UserResponse {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	repository, err := authrepositories.NewUserRepository(ctx, testConnection.Database)
	if err != nil {
		t.Fatalf("build user repository: %v", err)
	}

	user, err := repository.FindByEmail(ctx, email)
	if err != nil {
		t.Fatalf("find user by email: %v", err)
	}

	return apiresponses.UserResponse{
		ID:        user.ID,
		Email:     user.Email,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
	}
}

func MustFindUserDocument(t *testing.T, email string) bson.M {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var document bson.M
	if err := testConnection.Database.Collection("users").FindOne(
		ctx,
		bson.D{{Key: "email", Value: strings.ToLower(strings.TrimSpace(email))}},
	).Decode(&document); err != nil {
		t.Fatalf("find user document: %v", err)
	}

	return document
}

func MustFindSessionByToken(t *testing.T, token string) bson.M {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var document bson.M
	if err := testConnection.Database.Collection("sessions").FindOne(
		ctx,
		bson.D{{Key: "_id", Value: token}},
	).Decode(&document); err != nil {
		t.Fatalf("find session document: %v", err)
	}

	return document
}

func EventuallyFindDocument(t *testing.T, collection string, filter any) bson.M {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		var document bson.M
		lastErr = testConnection.Database.Collection(collection).FindOne(ctx, filter).Decode(&document)
		cancel()
		if lastErr == nil {
			return document
		}
		time.Sleep(50 * time.Millisecond)
	}

	t.Fatalf("find %s document before timeout: %v", collection, lastErr)
	return nil
}

func RequireMongoAvailable() {
	if os.Getenv("MONGO_URI") == "" {
		_ = os.Setenv("MONGO_URI", "mongodb://localhost:27017")
	}
}

func clearDatabase(ctx context.Context, connection *mongodb.Connection) error {
	if connection == nil {
		return fmt.Errorf("mongodb connection is nil")
	}

	return connection.Database.Drop(ctx)
}
