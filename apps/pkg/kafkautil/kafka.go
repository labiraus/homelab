package kafkautil

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/segmentio/kafka-go"
	"gopkg.in/yaml.v3"
)

var (
	config  KafkaConfig
	writers = make(map[string]*kafka.Writer)
	rwMux   = sync.RWMutex{}
)

type KafkaConfig struct {
	Brokers []string         `yaml:"brokers"`
	Topics  map[string]Topic `yaml:"topics"`
}

type Topic struct {
	Name                   string `yaml:"name"`
	GroupID                string `yaml:"groupId"`
	CreateTopic            bool   `yaml:"createTopic"`
	Partitions             int    `yaml:"partitions"`
	ReplicationFactor      int    `yaml:"replicationFactor"`
	QueueCapacity          int    `yaml:"queueCapacity"`
	BatchSize              int    `yaml:"batchSize"`
	BatchBytes             int64  `yaml:"batchBytes"`
	BatchTimeoutMs         int    `yaml:"batchTimeoutMs"`
	CommitIntervalMs       int    `yaml:"commitIntervalMs"`
	MinBytes               int    `yaml:"minBytes"`
	MaxBytes               int    `yaml:"maxBytes"`
	MaxWaitMs              int    `yaml:"maxWaitMs"`
	ReadLagIntervalSeconds int    `yaml:"readLagIntervalSeconds"`
}

func Start(ctx context.Context, c KafkaConfig) error {
	config = c
	if len(config.Brokers) == 0 {
		return fmt.Errorf("kafka brokers not configured")
	}
	slog.Info("initializing kafka", "brokers", strings.Join(config.Brokers, ","))

	conn, err := kafka.DialContext(ctx, "tcp", config.Brokers[0])
	if err != nil {
		return fmt.Errorf("kafka.DialContext: %w", err)
	}
	if err := conn.Close(); err != nil {
		return fmt.Errorf("closing kafka connection: %w", err)
	}

	return nil
}

func Subscribe(ctx context.Context, topicID string, handler func(context.Context, kafka.Message) error) error {
	topicConfig, err := getTopicConfig(topicID)
	if err != nil {
		return err
	}
	if topicConfig.GroupID == "" {
		return fmt.Errorf("topic %s missing groupId", topicID)
	}

	if err := ensureTopic(ctx, topicConfig); err != nil {
		return err
	}

	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:               config.Brokers,
		GroupID:               topicConfig.GroupID,
		Topic:                 topicConfig.Name,
		MinBytes:              defaultInt(topicConfig.MinBytes, 10e3),
		MaxBytes:              defaultInt(topicConfig.MaxBytes, 10e6),
		MaxWait:               time.Duration(defaultInt(topicConfig.MaxWaitMs, 1000)) * time.Millisecond,
		CommitInterval:        time.Duration(defaultInt(topicConfig.CommitIntervalMs, 0)) * time.Millisecond,
		ReadLagInterval:       time.Duration(defaultInt(topicConfig.ReadLagIntervalSeconds, -1)) * time.Second,
		QueueCapacity:         defaultInt(topicConfig.QueueCapacity, 100),
		WatchPartitionChanges: true,
	})

	go func() {
		defer func() {
			if err := reader.Close(); err != nil {
				slog.Error("error closing kafka reader", "topic", topicConfig.Name, "error", err)
			}
		}()
		for {
			msg, err := reader.ReadMessage(ctx)
			if err != nil {
				if errors.Is(err, context.Canceled) || ctx.Err() != nil {
					slog.Info("kafka reader stopped", "topic", topicConfig.Name, "groupId", topicConfig.GroupID)
					return
				}
				panic(fmt.Errorf("error reading kafka topic %s: %w", topicConfig.Name, err))
			}
			if err := handler(ctx, msg); err != nil {
				slog.Error("kafka handler failed", "topic", topicConfig.Name, "groupId", topicConfig.GroupID, "error", err)
			}
		}
	}()

	slog.Info("subscribed to kafka topic", "topic", topicConfig.Name, "groupId", topicConfig.GroupID)
	return nil
}

func GetWriter(ctx context.Context, topicID string) (*kafka.Writer, error) {
	topicConfig, err := getTopicConfig(topicID)
	if err != nil {
		return nil, err
	}

	var writer *kafka.Writer
	var ok bool
	func() {
		rwMux.RLock()
		defer rwMux.RUnlock()
		writer, ok = writers[topicConfig.Name]
	}()
	if ok {
		return writer, nil
	}

	if err := ensureTopic(ctx, topicConfig); err != nil {
		return nil, err
	}

	rwMux.Lock()
	defer rwMux.Unlock()

	if writer, ok = writers[topicConfig.Name]; ok {
		return writer, nil
	}

	writer = &kafka.Writer{
		Addr:         kafka.TCP(config.Brokers...),
		Topic:        topicConfig.Name,
		RequiredAcks: kafka.RequireAll,
		Async:        false,
		BatchSize:    defaultInt(topicConfig.BatchSize, 100),
		BatchBytes:   defaultInt64(topicConfig.BatchBytes, 1048576),
		BatchTimeout: time.Duration(defaultInt(topicConfig.BatchTimeoutMs, 1000)) * time.Millisecond,
	}
	writers[topicConfig.Name] = writer

	return writer, nil
}

func Publish(ctx context.Context, topicID string, key []byte, value []byte) error {
	writer, err := GetWriter(ctx, topicID)
	if err != nil {
		return err
	}

	return writer.WriteMessages(ctx, kafka.Message{
		Key:   key,
		Value: value,
		Time:  time.Now().UTC(),
	})
}

func ParseKafkaConfig(config map[string]string) (KafkaConfig, error) {
	var kafkaConfig KafkaConfig
	err := yaml.Unmarshal([]byte(config["kafka"]), &kafkaConfig)
	if err != nil {
		return KafkaConfig{}, err
	}
	return kafkaConfig, nil
}

func getTopicConfig(topicID string) (Topic, error) {
	topicConfig, ok := config.Topics[topicID]
	if !ok {
		return Topic{}, fmt.Errorf("topic %s not configured", topicID)
	}
	if topicConfig.Name == "" {
		return Topic{}, fmt.Errorf("topic %s missing name", topicID)
	}
	return topicConfig, nil
}

func ensureTopic(ctx context.Context, topicConfig Topic) error {
	conn, err := kafka.DialContext(ctx, "tcp", config.Brokers[0])
	if err != nil {
		return fmt.Errorf("dialing kafka broker: %w", err)
	}
	defer conn.Close()

	partitions, err := conn.ReadPartitions(topicConfig.Name)
	if err == nil && len(partitions) > 0 {
		return nil
	}
	if !topicConfig.CreateTopic {
		return fmt.Errorf("topic %s does not exist and cannot be created", topicConfig.Name)
	}

	controller, err := conn.Controller()
	if err != nil {
		return fmt.Errorf("getting kafka controller: %w", err)
	}
	controllerConn, err := kafka.DialContext(ctx, "tcp", fmt.Sprintf("%s:%d", controller.Host, controller.Port))
	if err != nil {
		return fmt.Errorf("dialing kafka controller: %w", err)
	}
	defer controllerConn.Close()

	err = controllerConn.CreateTopics(kafka.TopicConfig{
		Topic:             topicConfig.Name,
		NumPartitions:     defaultInt(topicConfig.Partitions, 1),
		ReplicationFactor: defaultInt(topicConfig.ReplicationFactor, 1),
	})
	if err != nil && !strings.Contains(err.Error(), "Topic with this name already exists") {
		return fmt.Errorf("creating kafka topic %s: %w", topicConfig.Name, err)
	}

	return nil
}

func defaultInt(value int, fallback int) int {
	if value == 0 {
		return fallback
	}
	return value
}

func defaultInt64(value int64, fallback int64) int64 {
	if value == 0 {
		return fallback
	}
	return value
}
