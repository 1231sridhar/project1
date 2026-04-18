apiVersion: admissionregistration.k8s.io/v1
kind: MutatingWebhookConfiguration
metadata:
  name: otel-sidecar-injector
  labels:
    app: otel-k8s-operator
    app.kubernetes.io/managed-by: otel-k8s-operator
webhooks:
  - name: otel-inject.sridharkancham.io
    admissionReviewVersions: ["v1", "v1beta1"]
    clientConfig:
      service:
        name: otel-operator-webhook-service
        namespace: observability
        path: "/mutate-pods"
    rules:
      - operations: ["CREATE"]
        apiGroups: [""]
        apiVersions: ["v1"]
        resources: ["pods"]
    namespaceSelector:
      matchLabels:
        otel-injection: enabled
    objectSelector:
      matchExpressions:
        - key: otel-inject
          operator: NotIn
          values: ["false", "disabled"]
    sideEffects: None
    failurePolicy: Ignore   # Don't block pod creation if webhook is down

---
# Webhook Service
apiVersion: v1
kind: Service
metadata:
  name: otel-operator-webhook-service
  namespace: observability
spec:
  selector:
    app: otel-k8s-operator
  ports:
    - port: 443
      targetPort: 9443
      name: webhook
