package mongoutil

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"gopkg.in/yaml.v3"
)

type MongoConfig struct {
	Host                     string `yaml:"host"`
	Port                     string `yaml:"port"`
	Database                 string `yaml:"database"`
	User                     string `yaml:"user"`
	Password                 string `yaml:"password"`
	AuthSource               string `yaml:"authSource"`
	ReplicaSet               string `yaml:"replicaSet"`
	Direct                   bool   `yaml:"direct"`
	ConnectTimeoutSeconds    int    `yaml:"connectTimeoutSeconds"`
	ServerSelectionTimeoutMS int    `yaml:"serverSelectionTimeoutMs"`
}

var (
	defaultDatabase *mongo.Database
	clients         = map[string]*mongo.Client{}
	databases       = map[string]*mongo.Database{}
	mu              sync.RWMutex
)

func Init(ctx context.Context, config map[string]MongoConfig) error {
	if len(config) == 0 {
		return fmt.Errorf("mongo config is empty")
	}

	var firstDatabase *mongo.Database
	for name, cfg := range config {
		database, err := initDatabase(ctx, name, cfg)
		if err != nil {
			return err
		}
		if firstDatabase == nil {
			firstDatabase = database
		}
	}

	defaultDatabase = firstDatabase
	return nil
}

func ParseMongoConfig(config map[string]string) (map[string]MongoConfig, error) {
	mongoConfig := make(map[string]MongoConfig, len(config))
	var mongoConfigValue MongoConfig
	for k, v := range config {
		err := yaml.Unmarshal([]byte(v), &mongoConfigValue)
		if err != nil {
			return nil, fmt.Errorf("could not unmarshal build config %v: %v", k, err)
		}
		mongoConfig[k] = mongoConfigValue
	}
	return mongoConfig, nil
}

func GetClient(name string) (*mongo.Client, error) {
	mu.RLock()
	defer mu.RUnlock()

	client, ok := clients[name]
	if !ok {
		return nil, fmt.Errorf("mongo client %s not initialized", name)
	}
	return client, nil
}

func GetDatabase(name string) (*mongo.Database, error) {
	mu.RLock()
	defer mu.RUnlock()

	database, ok := databases[name]
	if !ok {
		return nil, fmt.Errorf("mongo database %s not initialized", name)
	}
	return database, nil
}

func Collection(name string) (*mongo.Collection, error) {
	if defaultDatabase == nil {
		return nil, fmt.Errorf("mongo database not initialized")
	}
	return defaultDatabase.Collection(name), nil
}

func InsertOne(ctx context.Context, collectionName string, document any, opts ...options.Lister[options.InsertOneOptions]) (*mongo.InsertOneResult, error) {
	collection, err := Collection(collectionName)
	if err != nil {
		return nil, err
	}
	return collection.InsertOne(ctx, document, opts...)
}

func FindOne(ctx context.Context, collectionName string, filter any, opts ...options.Lister[options.FindOneOptions]) *mongo.SingleResult {
	collection, err := Collection(collectionName)
	if err != nil {
		return mongo.NewSingleResultFromDocument(bson.D{{Key: "error", Value: err.Error()}}, err, nil)
	}
	return collection.FindOne(ctx, filter, opts...)
}

func Find(ctx context.Context, collectionName string, filter any, opts ...options.Lister[options.FindOptions]) (*mongo.Cursor, error) {
	collection, err := Collection(collectionName)
	if err != nil {
		return nil, err
	}
	return collection.Find(ctx, filter, opts...)
}

func UpdateOne(ctx context.Context, collectionName string, filter any, update any, opts ...options.Lister[options.UpdateOneOptions]) (*mongo.UpdateResult, error) {
	collection, err := Collection(collectionName)
	if err != nil {
		return nil, err
	}
	return collection.UpdateOne(ctx, filter, update, opts...)
}

func DeleteOne(ctx context.Context, collectionName string, filter any, opts ...options.Lister[options.DeleteOneOptions]) (*mongo.DeleteResult, error) {
	collection, err := Collection(collectionName)
	if err != nil {
		return nil, err
	}
	return collection.DeleteOne(ctx, filter, opts...)
}

func CountDocuments(ctx context.Context, collectionName string, filter any, opts ...options.Lister[options.CountOptions]) (int64, error) {
	collection, err := Collection(collectionName)
	if err != nil {
		return 0, err
	}
	return collection.CountDocuments(ctx, filter, opts...)
}

func initDatabase(ctx context.Context, name string, config MongoConfig) (*mongo.Database, error) {
	if strings.TrimSpace(config.Host) == "" {
		return nil, fmt.Errorf("mongo config %s missing host", name)
	}
	if strings.TrimSpace(config.Database) == "" {
		return nil, fmt.Errorf("mongo config %s missing database", name)
	}

	uri := buildURI(config)
	clientOptions := options.Client().ApplyURI(uri)
	if config.ConnectTimeoutSeconds > 0 {
		clientOptions.SetConnectTimeout(time.Duration(config.ConnectTimeoutSeconds) * time.Second)
	}
	if config.ServerSelectionTimeoutMS > 0 {
		clientOptions.SetServerSelectionTimeout(time.Duration(config.ServerSelectionTimeoutMS) * time.Millisecond)
	}

	slog.Info("initializing mongo", "name", name, "host", config.Host, "port", defaultString(config.Port, "27017"), "database", config.Database)
	client, err := mongo.Connect(clientOptions)
	if err != nil {
		return nil, fmt.Errorf("could not create mongo client %s: %v", name, err)
	}

	if err := client.Ping(ctx, nil); err != nil {
		_ = client.Disconnect(ctx)
		return nil, fmt.Errorf("could not ping mongo %s: %v", name, err)
	}

	database := client.Database(config.Database)

	mu.Lock()
	clients[name] = client
	databases[name] = database
	mu.Unlock()

	go func() {
		<-ctx.Done()
		_ = client.Disconnect(context.Background())
	}()

	return database, nil
}

func buildURI(config MongoConfig) string {
	host := config.Host + ":" + defaultString(config.Port, "27017")
	credentials := ""
	if config.User != "" {
		credentials = url.QueryEscape(config.User)
		if config.Password != "" {
			credentials += ":" + url.QueryEscape(config.Password)
		}
		credentials += "@"
	}

	query := url.Values{}
	if config.AuthSource != "" {
		query.Set("authSource", config.AuthSource)
	}
	if config.ReplicaSet != "" {
		query.Set("replicaSet", config.ReplicaSet)
	}
	if config.Direct {
		query.Set("directConnection", "true")
	}

	uri := "mongodb://" + credentials + host
	if encodedQuery := query.Encode(); encodedQuery != "" {
		uri += "/?" + encodedQuery
	}
	return uri
}

func defaultString(value string, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
