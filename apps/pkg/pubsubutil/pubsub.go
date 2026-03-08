package pubsubutil

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync"

	"cloud.google.com/go/pubsub"
	"google.golang.org/api/option"
	"gopkg.in/yaml.v3"
)

var (
	client      *pubsub.Client
	topics      = make(map[string]*pubsub.Topic)
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

	topic := client.Topic(topicConfig.Name)
	if exists, err := topic.Exists(ctx); err != nil {
		return fmt.Errorf("failed to check if topic %s exists: %v", topicConfig.Name, err)
	} else if !exists && topicConfig.CreateTopic {
		topic, err = client.CreateTopic(ctx, topicConfig.Name)
		if err != nil {
			return fmt.Errorf("failed to create topic: %v", err)
		}
	} else if !exists {
		return fmt.Errorf("topic creation turned off %v and doesn't exist: %v", topicConfig.Name, err)
	}

	sub := client.Subscription(topicConfig.Subscription)
	if exists, err := sub.Exists(ctx); err != nil {
		return fmt.Errorf("failed to check if subscription %s exists: %v", topicConfig.Subscription, err)
	} else if !exists && topicConfig.CreateSubscription {
		sub, err = client.CreateSubscription(context.Background(), topicConfig.Subscription, pubsub.SubscriptionConfig{Topic: topic})
		if err != nil {
			return fmt.Errorf("failed to subscribe to topic %v as subscription %v: %v", topicConfig.Name, topicConfig.Subscription, err)
		}
	} else if !exists {
		return fmt.Errorf("subscription creation turned off and %v on topic %v doesn't exist: %v", topicConfig.Subscription, topicConfig.Name, err)
	}

	if topicConfig.Concurrency > 1 {
		sub.ReceiveSettings.Synchronous = false
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

func GetTopic(ctx context.Context, topicID string) (*pubsub.Topic, error) {
	topicConfig := config.Topics[topicID]
	var topic *pubsub.Topic
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
		topic = client.Topic(topicConfig.Name)
		if exists, err := topic.Exists(ctx); err != nil {
			return fmt.Errorf("failed to check if topic %s exists: %v", topicConfig.Name, err)
		} else if !exists && topicConfig.CreateTopic {
			topic, err = client.CreateTopic(ctx, topicConfig.Name)
			if err != nil {
				return fmt.Errorf("failed to create topic: %v", err)
			}
		} else if !exists {
			return fmt.Errorf("topic does not exist and cannot be created")
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
