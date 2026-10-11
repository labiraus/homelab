// A real disposable API server for source validation. Never loads kubeconfig.
package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"syscall"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/restmapper"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	sigyaml "sigs.k8s.io/yaml"
)

type document struct {
	file   string
	object *unstructured.Unstructured
}

func documents(directory string, patterns ...string) ([]document, error) {
	var result []document
	for _, pattern := range patterns {
		paths, err := filepath.Glob(filepath.Join(directory, pattern))
		if err != nil {
			return nil, err
		}
		for _, path := range paths {
			file, err := os.Open(path)
			if err != nil {
				return nil, err
			}
			reader := yaml.NewYAMLReader(bufio.NewReader(file))
			for {
				object := &unstructured.Unstructured{}
				raw, readErr := reader.Read()
				err = readErr
				if errors.Is(err, io.EOF) {
					break
				}
				if err != nil {
					file.Close()
					return nil, fmt.Errorf("%s: %w", path, err)
				}
				body, err := sigyaml.YAMLToJSONStrict(raw)
				if err != nil {
					file.Close()
					return nil, fmt.Errorf("%s: %w", path, err)
				}
				if bytes.Equal(bytes.TrimSpace(body), []byte("null")) {
					continue
				}
				if err = object.UnmarshalJSON(body); err != nil {
					file.Close()
					return nil, fmt.Errorf("%s: %w", path, err)
				}
				if len(object.Object) != 0 {
					result = append(result, document{filepath.Base(path), object})
				}
			}
			if err = file.Close(); err != nil {
				return nil, err
			}
		}
	}
	return result, nil
}

func checkedAssets(directory string) error {
	data, err := os.ReadFile(filepath.Join(directory, "assets.json"))
	if err != nil {
		return err
	}
	var manifest struct {
		Version  string
		Binaries map[string]string
	}
	if err = json.Unmarshal(data, &manifest); err != nil {
		return err
	}
	if manifest.Version != "1.35.0" {
		return fmt.Errorf("expected pinned envtest 1.35.0")
	}
	for _, binary := range []string{"kube-apiserver", "etcd", "kubectl"} {
		file, err := os.Open(filepath.Join(directory, binary))
		if err != nil {
			return err
		}
		hash := sha256.New()
		_, readErr := io.Copy(hash, file)
		closeErr := file.Close()
		if readErr != nil {
			return readErr
		}
		if closeErr != nil {
			return closeErr
		}
		if hex.EncodeToString(hash.Sum(nil)) != manifest.Binaries[binary] {
			return fmt.Errorf("test binary checksum differs: %s", binary)
		}
	}
	return nil
}

var crdResource = schema.GroupVersionResource{Group: "apiextensions.k8s.io", Version: "v1", Resource: "customresourcedefinitions"}

func installCRD(ctx context.Context, client dynamic.Interface, object *unstructured.Unstructured) error {
	// A remote conversion webhook could leave the fixture network. Such source
	// CRDs need a separately configured local webhook, never silent rewriting.
	strategy, _, err := unstructured.NestedString(object.Object, "spec", "conversion", "strategy")
	if err != nil {
		return err
	}
	if strategy != "" && strategy != "None" {
		return fmt.Errorf("%s requires an explicit local conversion webhook fixture", object.GetName())
	}
	if _, err = client.Resource(crdResource).Create(ctx, object, metav1.CreateOptions{FieldValidation: "Strict"}); err != nil {
		return err
	}
	return wait.PollUntilContextTimeout(ctx, 100*time.Millisecond, 30*time.Second, true, func(ctx context.Context) (bool, error) {
		current, err := client.Resource(crdResource).Get(ctx, object.GetName(), metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		rawConditions, _, err := unstructured.NestedFieldNoCopy(current.Object, "status", "conditions")
		if err != nil {
			return false, err
		}
		if rawConditions == nil {
			return false, nil
		}
		conditions, _, err := unstructured.NestedSlice(current.Object, "status", "conditions")
		if err != nil {
			return false, err
		}
		for _, raw := range conditions {
			condition := raw.(map[string]interface{})
			if condition["type"] == "NamesAccepted" && condition["status"] == "False" {
				return false, fmt.Errorf("CRD names rejected: %s", object.GetName())
			}
			if condition["type"] == "Established" && condition["status"] == "True" {
				return true, nil
			}
		}
		return false, nil
	})
}

func object(data string) *unstructured.Unstructured {
	value := &unstructured.Unstructured{}
	if err := value.UnmarshalJSON([]byte(data)); err != nil {
		panic(err)
	}
	return value
}

func refusalChecks(ctx context.Context, client dynamic.Interface) error {
	service := object(`{"apiVersion":"v1","kind":"Service","metadata":{"name":"invalid-port","namespace":"default"},"spec":{"ports":[{"port":70000}]}}`)
	_, err := client.Resource(schema.GroupVersionResource{Version: "v1", Resource: "services"}).Namespace("default").Create(ctx, service, metav1.CreateOptions{DryRun: []string{"All"}, FieldValidation: "Strict"})
	if !apierrors.IsInvalid(err) || !strings.Contains(err.Error(), "port") {
		return fmt.Errorf("built-in invalid port was not refused: %v", err)
	}
	fixture := object(`{"apiVersion":"apiextensions.k8s.io/v1","kind":"CustomResourceDefinition","metadata":{"name":"ranges.acceptance.homelab.invalid"},"spec":{"group":"acceptance.homelab.invalid","scope":"Namespaced","names":{"plural":"ranges","singular":"range","kind":"Range"},"versions":[{"name":"v1","served":true,"storage":true,"schema":{"openAPIV3Schema":{"type":"object","properties":{"spec":{"type":"object","required":["minimum","maximum"],"properties":{"minimum":{"type":"integer"},"maximum":{"type":"integer"}},"x-kubernetes-validations":[{"rule":"self.minimum <= self.maximum","message":"minimum must not exceed maximum"}]}}}}}]}}`)
	if err = installCRD(ctx, client, fixture); err != nil {
		return fmt.Errorf("CEL fixture: %w", err)
	}
	resource := client.Resource(schema.GroupVersionResource{Group: "acceptance.homelab.invalid", Version: "v1", Resource: "ranges"}).Namespace("default")
	valid := object(`{"apiVersion":"acceptance.homelab.invalid/v1","kind":"Range","metadata":{"name":"valid"},"spec":{"minimum":1,"maximum":2}}`)
	if _, err = resource.Create(ctx, valid, metav1.CreateOptions{DryRun: []string{"All"}, FieldValidation: "Strict"}); err != nil {
		return err
	}
	if _, err = resource.Get(ctx, "valid", metav1.GetOptions{}); !apierrors.IsNotFound(err) {
		return fmt.Errorf("dry run persisted fixture: %v", err)
	}
	invalid := valid.DeepCopy()
	unstructured.SetNestedField(invalid.Object, int64(3), "spec", "minimum")
	_, err = resource.Create(ctx, invalid, metav1.CreateOptions{DryRun: []string{"All"}, FieldValidation: "Strict"})
	if !apierrors.IsInvalid(err) || !strings.Contains(err.Error(), "minimum must not exceed maximum") {
		return fmt.Errorf("CEL contradiction was not refused: %v", err)
	}
	unknown := valid.DeepCopy()
	unstructured.SetNestedField(unknown.Object, "unexpected", "spec", "unknown")
	_, err = resource.Create(ctx, unknown, metav1.CreateOptions{DryRun: []string{"All"}, FieldValidation: "Strict"})
	if err == nil || !strings.Contains(err.Error(), "unknown field") {
		return fmt.Errorf("strict unknown-field validation did not refuse fixture: %v", err)
	}
	fmt.Println("PASS: real built-in invalid-port, CEL contradiction and unknown-field refusal; dry runs do not persist")
	return nil
}

func run(ctx context.Context, assets, schemaDir, renderDir string) (result error) {
	if err := checkedAssets(assets); err != nil {
		return err
	}
	source, err := documents(schemaDir, "*.yaml")
	if err != nil {
		return err
	}
	renders, err := documents(renderDir, "*.template", "*-enabled.yaml")
	if err != nil {
		return err
	}
	if len(renders) == 0 {
		return fmt.Errorf("no rendered resources")
	}
	crds := map[string]*unstructured.Unstructured{}
	for _, doc := range source {
		if doc.object.GetKind() != "CustomResourceDefinition" {
			continue
		}
		name := doc.object.GetName()
		if prior, ok := crds[name]; ok && !reflect.DeepEqual(prior.Object["spec"], doc.object.Object["spec"]) {
			return fmt.Errorf("conflicting source CRD %s", name)
		}
		crds[name] = doc.object
	}
	if len(crds) == 0 {
		return fmt.Errorf("no source CRDs")
	}
	// Explicit false takes precedence over USE_EXISTING_CLUSTER. Explicit paths
	// bypass TEST_ASSET_* and KUBEBUILDER_ASSETS; no kubeconfig is read or written.
	useExisting := false
	testEnv := &envtest.Environment{UseExistingCluster: &useExisting, ControlPlaneStartTimeout: 60 * time.Second, ControlPlaneStopTimeout: 30 * time.Second}
	apiServer := testEnv.ControlPlane.GetAPIServer()
	apiServer.Path = filepath.Join(assets, "kube-apiserver")
	apiServer.SecureServing.Address = "127.0.0.1"
	apiServer.Configure().Set("bind-address", "127.0.0.1").Set("advertise-address", "127.0.0.1")
	testEnv.ControlPlane.Etcd = &envtest.Etcd{Path: filepath.Join(assets, "etcd")}
	testEnv.ControlPlane.KubectlPath = filepath.Join(assets, "kubectl")
	defer func() { result = errors.Join(result, testEnv.Stop()) }()
	cfg, err := testEnv.Start()
	if err != nil {
		return fmt.Errorf("disposable control plane: %w", err)
	}
	address, err := url.Parse(cfg.Host)
	if err != nil || !net.ParseIP(address.Hostname()).IsLoopback() {
		return fmt.Errorf("refusing non-loopback test control plane")
	}
	cfg.Proxy = func(*http.Request) (*url.URL, error) { return nil, nil }
	cfg.Timeout = 30 * time.Second
	cfg.QPS, cfg.Burst = 50, 100
	client, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return err
	}
	names := make([]string, 0, len(crds))
	for name := range crds {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if err = installCRD(ctx, client, crds[name]); err != nil {
			return fmt.Errorf("source CRD %s: %w", name, err)
		}
	}
	fmt.Printf("PASS: %d unmodified source CRDs installed in disposable Kubernetes 1.35.0\n", len(crds))
	if err = refusalChecks(ctx, client); err != nil {
		return err
	}
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(cfg)
	if err != nil {
		return err
	}
	resources, err := restmapper.GetAPIGroupResources(discoveryClient)
	if err != nil {
		return err
	}
	mapper := restmapper.NewDiscoveryRESTMapper(resources)
	namespaces := map[string]bool{"default": true, "kube-system": true, "kube-public": true, "kube-node-lease": true}
	for _, doc := range renders {
		namespace := doc.object.GetNamespace()
		if namespace == "" || namespaces[namespace] {
			continue
		}
		ns := object(`{"apiVersion":"v1","kind":"Namespace"}`)
		ns.SetName(namespace)
		if _, err = client.Resource(schema.GroupVersionResource{Version: "v1", Resource: "namespaces"}).Create(ctx, ns, metav1.CreateOptions{}); err != nil && !apierrors.IsAlreadyExists(err) {
			return err
		}
		namespaces[namespace] = true
	}
	builtins, custom, failures := 0, 0, []string{}
	for _, doc := range renders {
		value := doc.object.DeepCopy()
		mapping, err := mapper.RESTMapping(value.GroupVersionKind().GroupKind(), value.GroupVersionKind().Version)
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s %s: missing API/schema: %v", doc.file, value.GetKind(), err))
			continue
		}
		namespace := value.GetNamespace()
		if mapping.Scope.Name() == "namespace" && namespace == "" {
			namespace = "default"
		}
		var resource dynamic.ResourceInterface = client.Resource(mapping.Resource)
		if mapping.Scope.Name() == "namespace" {
			resource = client.Resource(mapping.Resource).Namespace(namespace)
		}
		// Existing fixture namespaces are updated via dry-run Apply; other rendered
		// objects are never persisted and each overlay is validated independently.
		body, marshalErr := json.Marshal(value.Object)
		if marshalErr != nil {
			return marshalErr
		}
		force := true
		_, err = resource.Patch(ctx, value.GetName(), types.ApplyPatchType, body, metav1.PatchOptions{DryRun: []string{"All"}, FieldManager: "offline-acceptance", Force: &force, FieldValidation: "Strict"})
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s %s/%s: %v", doc.file, value.GetKind(), value.GetName(), err))
			continue
		}
		if _, ok := crds[mapping.Resource.Resource+"."+mapping.Resource.Group]; ok {
			custom++
		} else {
			builtins++
		}
	}
	for _, failure := range failures {
		fmt.Println("FAIL:", failure)
	}
	if len(failures) > 0 {
		return fmt.Errorf("%d rendered resources refused", len(failures))
	}
	fmt.Printf("PASS: %d rendered resources accepted (%d built-in, %d custom); no workload/operator processes started\n", len(renders), builtins, custom)
	return nil
}

func main() {
	if len(os.Args) != 4 {
		fmt.Fprintln(os.Stderr, "Usage: platform-acceptance ASSETS_DIR SCHEMA_DIR RENDER_DIR")
		os.Exit(2)
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if err := run(ctx, os.Args[1], os.Args[2], os.Args[3]); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL:", err)
		os.Exit(1)
	}
}
