package pubsubutil

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync"

	"cloud.google.com/go/pubsub/v2"
	"cloud.google.com/go/pubsub/v2/apiv1/pubsubpb"
	"google.golang.org/api/option"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gopkg.in/yaml.v3"
)

var (
	client      *pubsub.Client
	topics      = make(map[string]*pubsub.Publisher)
	UseProtobuf = false
	config      PubsubConfig
	rwMux       = sync.RWMutex{}
)

type PubsubConfig struct {
	Host      string           `yaml:"host"`
	Port      string           `yaml:"port"`
	Projectid string           `yaml:"projectid"`
	Emulator  bool             `yaml:"emulator"`
	Topics    map[string]Topic `yaml:"topics"`
}

type Topic struct {
	Name                   string `yaml:"name"`
	Subscription           string `yaml:"subscription"`
	CreateTopic            bool   `yaml:"createTopic"`
	CreateSubscription     bool   `yaml:"createSubscription"`
	Concurrency            int    `yaml:"concurrency"`
	MaxOutstandingMessages int    `yaml:"maxOutstandingMessages"`
}

func Start(ctx context.Context, c PubsubConfig) error {
	config = c
	var err error
	slog.Info("initializing pubsub", "config", config)
	opts := []option.ClientOption{
		option.WithGRPCConnectionPool(2),
	}
	if config.Emulator {
		os.Setenv("PUBSUB_EMULATOR_HOST", "pubsub:"+config.Port)
		opts = append(opts, option.WithoutAuthentication())
		opts = append(opts, option.WithEndpoint(config.Host+":"+config.Port))
	}
	client, err = pubsub.NewClient(ctx, config.Projectid, opts...)
	if err != nil {
		return fmt.Errorf("pubsub.NewClient: %w", err)
	}
	go func() {
		defer client.Close()
		<-ctx.Done()
		slog.Info("pubsub client closed")
	}()

	return nil
}

func Subscribe(ctx context.Context, topicID string, handler func(context.Context, *pubsub.Message)) error {
	topicConfig := config.Topics[topicID]

	topic, err := GetTopic(ctx, topicID)
	if err != nil {
		return err
	}
	sub := client.Subscriber(topicConfig.Subscription)
	_, err = client.SubscriptionAdminClient.GetSubscription(ctx, &pubsubpb.GetSubscriptionRequest{Subscription: sub.String()})
	if status.Code(err) == codes.NotFound && topicConfig.CreateSubscription {
		_, err = client.SubscriptionAdminClient.CreateSubscription(ctx, &pubsubpb.Subscription{Name: sub.String(), Topic: topic.String()})
	}
	if err != nil {
		return fmt.Errorf("failed to access subscription %s: %w", topicConfig.Subscription, err)
	}

	if topicConfig.Concurrency > 1 {
		sub.ReceiveSettings.NumGoroutines = topicConfig.Concurrency
		sub.ReceiveSettings.MaxOutstandingMessages = topicConfig.MaxOutstandingMessages
	}

	go func() {
		err := sub.Receive(ctx, handler)
		if err != nil {
			panic(fmt.Errorf("error recieving subscription: %v", err))
		}
	}()
	slog.Info(fmt.Sprintf("subscribed to topic [%v] as subscription [%v]", topicConfig.Name, topicConfig.Subscription))

	return nil
}

func GetTopic(ctx context.Context, topicID string) (*pubsub.Publisher, error) {
	topicConfig := config.Topics[topicID]
	var topic *pubsub.Publisher
	ok := false
	func() {
		rwMux.RLock()
		defer rwMux.RUnlock()
		topic, ok = topics[topicConfig.Name]
	}()
	if ok {
		return topic, nil
	}

	err := func() error {
		rwMux.Lock()
		defer rwMux.Unlock()
		topic = client.Publisher(topicConfig.Name)
		_, err := client.TopicAdminClient.GetTopic(ctx, &pubsubpb.GetTopicRequest{Topic: topic.String()})
		if status.Code(err) == codes.NotFound && topicConfig.CreateTopic {
			_, err = client.TopicAdminClient.CreateTopic(ctx, &pubsubpb.Topic{Name: topic.String()})
		}
		if err != nil {
			return fmt.Errorf("failed to access topic %s: %w", topicConfig.Name, err)
		}
		topics[topicConfig.Name] = topic
		return nil
	}()

	return topic, err
}

func ParsePubsubConfig(config map[string]string) (PubsubConfig, error) {
	var pubsubConfigValue PubsubConfig
	err := yaml.Unmarshal([]byte(config["pubsub"]), &pubsubConfigValue)
	if err != nil {
		return PubsubConfig{}, err
	}
	return pubsubConfigValue, nil
}
