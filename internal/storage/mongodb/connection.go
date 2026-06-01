package mongodb

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Connection struct {
	Client   *mongo.Client
	Database *mongo.Database
}

func Connect(ctx context.Context, uri, databaseName string) (*Connection, error) {
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		return nil, fmt.Errorf("connect to mongodb: %w", err)
	}

	if err := client.Ping(ctx, nil); err != nil {
		_ = client.Disconnect(context.Background())
		return nil, fmt.Errorf("ping mongodb: %w", err)
	}

	return &Connection{
		Client:   client,
		Database: client.Database(databaseName),
	}, nil
}

func (c *Connection) Close(ctx context.Context) error {
	return c.Client.Disconnect(ctx)
}
