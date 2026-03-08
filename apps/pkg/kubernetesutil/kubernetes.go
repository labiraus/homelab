package kubernetesutil

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

var (
	namespace string
	clientset *kubernetes.Clientset
)

func Start() (bool, error) {
	namespace = os.Getenv("namespace")
	if namespace == "" {
		return false, nil
	}
	config, err := rest.InClusterConfig()
	if err != nil {
		return false, fmt.Errorf("could not get in-cluster config: %v", err)
	}

	clientset, err = kubernetes.NewForConfig(config)
	if err != nil {
		return false, fmt.Errorf("could not create kubernetes client: %v", err)
	}

	return true, nil
}

func GetConfigWithRetry(ctx context.Context, configName string) (map[string]string, error) {
	var err error
	var configMap *v1.ConfigMap
	for attempt := 0; attempt < 5; attempt++ {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("failure %v: %v", attempt, r)
			}
		}()
		configMap, err = clientset.CoreV1().ConfigMaps(namespace).Get(ctx, configName, metav1.GetOptions{})
		if err == nil {
			return configMap.Data, nil
		}
		slog.Error(fmt.Sprintf("failed to get config map attempt %v: %v", attempt, err))
		time.Sleep(1 * time.Second)
	}
	return nil, fmt.Errorf("could not load build config: %v", err)
}

func GetSecret(ctx context.Context, secretName string) (map[string][]byte, error) {
	if clientset == nil {
		return nil, fmt.Errorf("kubernetes client not initialized")
	}
	secret, err := clientset.CoreV1().Secrets(namespace).Get(ctx, secretName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("could not get secret: %v", err)
	}
	return secret.Data, nil
}
