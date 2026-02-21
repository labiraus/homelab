cilium = {
  enabled                = true
  chart_version          = "1.19.1"
  namespace              = "kube-system"
  kube_proxy_replacement = "strict"
  k8s_service_host       = "192.168.8.10"
  k8s_service_port       = 6443
  values = {
    "ipam.mode"           = "kubernetes"
    "operator.replicas"   = "1"
    "rollOutCiliumPods"   = "true"
  }
}
