package database

import (
	"context"
	"fmt"
	"time"

	"hero-quest/pkg/logger"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"
)

// MongoDB 封装 mongo.Client 和 mongo.Database，与 DB 平级。
type MongoDB struct {
	Client   *mongo.Client
	Database *mongo.Database
}

// NewMongo 连接 MongoDB 并返回实例。
func NewMongo(uri, database string) (*MongoDB, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(options.Client().ApplyURI(uri))
	if err != nil {
		return nil, fmt.Errorf("mongo connect: %w", err)
	}

	if err := client.Ping(ctx, readpref.Primary()); err != nil {
		return nil, fmt.Errorf("mongo ping: %w", err)
	}

	logger.Info("mongodb connected", "uri", uri, "database", database)
	return &MongoDB{
		Client:   client,
		Database: client.Database(database),
	}, nil
}

// Close 优雅关闭 MongoDB 连接。
func (m *MongoDB) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return m.Client.Disconnect(ctx)
}
