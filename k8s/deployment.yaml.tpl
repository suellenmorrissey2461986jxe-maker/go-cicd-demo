apiVersion: apps/v1
kind: Deployment
metadata:
  name: go-cicd-demo
  namespace: go-cicd-demo
spec:
  replicas: 2

  revisionHistoryLimit: 5
  progressDeadlineSeconds: 180
  minReadySeconds: 5

  strategy:
    type: RollingUpdate
    rollingUpdate:
      maxUnavailable: 0
      maxSurge: 1

  selector:
    matchLabels:
      app: go-cicd-demo

  template:
    metadata:
      labels:
        app: go-cicd-demo

    spec:
      imagePullSecrets:
        - name: harbor-regcred

      securityContext:
        runAsNonRoot: true
        seccompProfile:
          type: RuntimeDefault

      affinity:
        nodeAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
            nodeSelectorTerms:
              - matchExpressions:
                  - key: workload
                    operator: In
                    values:
                      - apps

        podAntiAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
            - labelSelector:
                matchExpressions:
                  - key: app
                    operator: In
                    values:
                      - go-cicd-demo
              topologyKey: kubernetes.io/hostname
              matchLabelKeys:
                - pod-template-hash

      topologySpreadConstraints:
        - maxSkew: 1
          topologyKey: kubernetes.io/hostname
          whenUnsatisfiable: DoNotSchedule
          matchLabelKeys:
            - pod-template-hash
          labelSelector:
            matchLabels:
              app: go-cicd-demo

      containers:
        - name: app
          image: __IMAGE_REFERENCE__
          imagePullPolicy: IfNotPresent

          ports:
            - name: http
              containerPort: 8080
              protocol: TCP

          readinessProbe:
            httpGet:
              path: /healthz
              port: http
              scheme: HTTP
            initialDelaySeconds: 2
            periodSeconds: 5
            timeoutSeconds: 2
            failureThreshold: 3
            successThreshold: 1

          livenessProbe:
            httpGet:
              path: /healthz
              port: http
              scheme: HTTP
            initialDelaySeconds: 10
            periodSeconds: 10
            timeoutSeconds: 2
            failureThreshold: 3
            successThreshold: 1

          resources:
            requests:
              cpu: 50m
              memory: 32Mi
            limits:
              cpu: 200m
              memory: 128Mi

          securityContext:
            allowPrivilegeEscalation: false
            readOnlyRootFilesystem: true
            capabilities:
              drop:
                - ALL
