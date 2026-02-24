# Disable swap
sudo swapoff -a
sudo sed -i.bak '/\sswap\s/s/^/#/' /etc/fstab

# Load required kernel modules
cat <<'EOF' | sudo tee /etc/modules-load.d/k8s.conf
overlay
br_netfilter
EOF

sudo modprobe overlay
sudo modprobe br_netfilter

# Configure sysctl for Kubernetes networking
cat <<'EOF' | sudo tee /etc/sysctl.d/k8s.conf
net.bridge.bridge-nf-call-iptables  = 1
net.bridge.bridge-nf-call-ip6tables = 1
net.ipv4.ip_forward                 = 1
EOF

sudo sysctl --system

# Install containerd
sudo apt-get update
sudo apt-get install -y containerd
sudo mkdir -p /etc/containerd
sudo containerd config default | sudo tee /etc/containerd/config.toml >/dev/null
sudo sed -i 's/SystemdCgroup = false/SystemdCgroup = true/' /etc/containerd/config.toml
sudo systemctl restart containerd
sudo systemctl enable containerd

# Add Kubernetes 1.34 repo
sudo mkdir -p /etc/apt/keyrings
curl -fsSL https://pkgs.k8s.io/core:/stable:/v1.34/deb/Release.key \
  | sudo gpg --dearmor -o /etc/apt/keyrings/kubernetes-apt-keyring.gpg

echo "deb [signed-by=/etc/apt/keyrings/kubernetes-apt-keyring.gpg] \
https://pkgs.k8s.io/core:/stable:/v1.34/deb/ /" \
| sudo tee /etc/apt/sources.list.d/kubernetes.list

sudo apt update
sudo apt install -y kubelet kubeadm kubectl
sudo apt-mark hold kubelet kubeadm kubectl
sudo systemctl enable kubelet

# Initialize control plane
sudo kubeadm init --pod-network-cidr=10.244.0.0/16

# Configure kubectl
mkdir -p $HOME/.kube
sudo cp /etc/kubernetes/admin.conf $HOME/.kube/config
sudo chown $(id -u):$(id -g) $HOME/.kube/config

# Install Cilium CLI
CILIUM_CLI_VERSION=$(curl -s https://raw.githubusercontent.com/cilium/cilium-cli/main/stable.txt)
CLI_ARCH=amd64
case "$(uname -m)" in
  aarch64|arm64) CLI_ARCH=arm64 ;;
  x86_64|amd64) CLI_ARCH=amd64 ;;
esac

curl -L --fail --remote-name-all \
  "https://github.com/cilium/cilium-cli/releases/download/${CILIUM_CLI_VERSION}/cilium-linux-${CLI_ARCH}.tar.gz" \
  "https://github.com/cilium/cilium-cli/releases/download/${CILIUM_CLI_VERSION}/cilium-linux-${CLI_ARCH}.tar.gz.sha256sum"
sha256sum --check "cilium-linux-${CLI_ARCH}.tar.gz.sha256sum"
sudo tar xzvfC "cilium-linux-${CLI_ARCH}.tar.gz" /usr/local/bin
rm "cilium-linux-${CLI_ARCH}.tar.gz" "cilium-linux-${CLI_ARCH}.tar.gz.sha256sum"

# Install Cilium CNI (matches kubeadm --pod-network-cidr)
cilium install --set ipam.operator.clusterPoolIPv4PodCIDRList="10.244.0.0/16"
cilium status --wait

# Install Istio CLI (1.29.x)
curl -L https://istio.io/downloadIstio | ISTIO_VERSION=1.29.0 sh -
cd istio-*
sudo install -m 0755 bin/istioctl /usr/local/bin/istioctl

# Install Istio Ambient, but disable Istio CNI node agent (istio-cni-node DaemonSet)

# 1) generate the install manifest (no changes applied yet)
istioctl manifest generate \
  --set profile=ambient \
  --set components.cni.enabled=true \
  --set values.cni.cniBinDir=/opt/cni/bin \
  --set values.cni.cniConfDir=/etc/cni/net.d \
  > /tmp/istio-ambient-manifest.yaml

# 2) apply the generated YAML immediately (this won't block waiting for readiness)
kubectl apply -f /tmp/istio-ambient-manifest.yaml

# 3) restrict istio data-plane pieces to labeled worker nodes (so control plane stays clean)
# (make sure you've labeled your workers: kubectl label node <worker> node.mesh=enabled)
kubectl -n istio-system patch ds istio-cni-node --type='merge' -p '
{"spec":{"template":{"spec":{"nodeSelector":{"node.mesh":"enabled"}}}}}'

kubectl -n istio-system patch ds ztunnel --type='merge' -p '
{"spec":{"template":{"spec":{"nodeSelector":{"node.mesh":"enabled"}}}}}'

# 4) restart the DaemonSets so they immediately reconcile against the new nodeSelector
kubectl -n istio-system rollout restart ds/istio-cni-node ds/ztunnel || true

# 5) now wait — do the "blocking" checks here at the end (tweak timeouts to taste)
# wait for the daemonsets to be updated
kubectl -n istio-system rollout status ds/istio-cni-node --timeout=5m || echo "istio-cni-node not ready (continue to check other resources)"
kubectl -n istio-system rollout status ds/ztunnel --timeout=5m || echo "ztunnel not ready (continue to check other resources)"

# wait for core control-plane deployments
kubectl -n istio-system rollout status deploy/istiod --timeout=5m || echo "istiod not ready"
