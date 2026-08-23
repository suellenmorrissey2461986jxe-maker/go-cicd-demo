pipeline {

    agent {
        kubernetes {
            yaml '''
apiVersion: v1
kind: Pod
spec:
  serviceAccountName: jenkins-deployer

  containers:

  - name: jnlp
    image: jenkins/inbound-agent:3384.v60d89463d9e0-2-jdk25
    env:
    - name: HTTP_PROXY
      value: "http://100.69.97.15:7890"
    - name: HTTPS_PROXY
      value: "http://100.69.97.15:7890"
    - name: NO_PROXY
      value: "127.0.0.1,localhost,10.42.0.0/16,10.43.0.0/16,100.64.0.0/10,.svc,.cluster.local"
    - name: http_proxy
      value: "http://100.69.97.15:7890"
    - name: https_proxy
      value: "http://100.69.97.15:7890"
    - name: no_proxy
      value: "127.0.0.1,localhost,10.42.0.0/16,10.43.0.0/16,100.64.0.0/10,.svc,.cluster.local"

  - name: golang
    image: golang:1.22
    command:
    - sleep
    args:
    - 99d
    env:
    - name: HTTP_PROXY
      value: "http://100.69.97.15:7890"
    - name: HTTPS_PROXY
      value: "http://100.69.97.15:7890"
    - name: NO_PROXY
      value: "127.0.0.1,localhost,10.42.0.0/16,10.43.0.0/16,100.64.0.0/10,.svc,.cluster.local"

  - name: buildkit
    image: moby/buildkit:v0.32.2-rootless
    imagePullPolicy: IfNotPresent
    command:
    - sleep
    args:
    - 99d
    env:
    - name: BUILDKITD_FLAGS
      value: "--oci-worker-no-process-sandbox --oci-worker-snapshotter=native"
    - name: HTTP_PROXY
      value: "http://100.69.97.15:7890"
    - name: HTTPS_PROXY
      value: "http://100.69.97.15:7890"
    - name: NO_PROXY
      value: "127.0.0.1,localhost,10.42.0.0/16,10.43.0.0/16,100.64.0.0/10,.svc,.cluster.local"
    securityContext:
      runAsUser: 1000
      runAsGroup: 1000
      seccompProfile:
        type: Unconfined
      appArmorProfile:
        type: Unconfined
    resources:
      requests:
        cpu: 100m
        memory: 256Mi
      limits:
        memory: 1Gi
    volumeMounts:
    - name: buildkit-cache
      mountPath: /home/user/.local/share/buildkit

  - name: kubectl
    image: alpine/kubectl:1.36.3
    imagePullPolicy: IfNotPresent
    command:
    - sleep
    args:
    - 99d
    resources:
      requests:
        cpu: 50m
        memory: 64Mi
      limits:
        memory: 128Mi

  volumes:
  - name: buildkit-cache
    emptyDir: {}
'''
        }
    }

    options {
        skipDefaultCheckout(true)
        disableConcurrentBuilds()
    }

    triggers {
        pollSCM('H/2 * * * *')
    }

    environment {
        IMAGE_REPO = 'ghcr.io/suellenmorrissey2461986jxe-maker/go-cicd-demo'
    }

    stages {

        stage('Checkout') {
            steps {
                retry(3) {
                    checkout scm
                }
            }
        }

        stage('Go Test') {
            steps {
                container('golang') {
                    sh '''
                    set -eu

                    go version
                    go test ./...
                    '''
                }
            }
        }

        stage('Go Build') {
            steps {
                container('golang') {
                    sh '''
                    set -eu

                    git config --global --add safe.directory "$WORKSPACE"

                    go build \
                        -trimpath \
                        -ldflags="-X main.version=${BUILD_NUMBER}" \
                        -o app .

                    ls -lh app
                    '''
                }
            }
        }

        stage('Build and Push Image') {
            steps {
                container('buildkit') {
                    withCredentials([
                        usernamePassword(
                            credentialsId: 'ghcr-credentials',
                            usernameVariable: 'GHCR_USER',
                            passwordVariable: 'GHCR_TOKEN'
                        )
                    ]) {
                        sh '''
                        set -eu
                        set +x

                        export DOCKER_CONFIG=/home/user/.docker
                        mkdir -p "$DOCKER_CONFIG"

                        AUTH="$(printf '%s:%s' "$GHCR_USER" "$GHCR_TOKEN" \
                            | base64 | tr -d '\n')"

                        printf '{"auths":{"ghcr.io":{"auth":"%s"}}}\n' "$AUTH" \
                            > "$DOCKER_CONFIG/config.json"

                        unset AUTH
                        trap 'rm -f "$DOCKER_CONFIG/config.json"' EXIT

                        set -x

                        buildctl-daemonless.sh build \
                            --frontend dockerfile.v0 \
                            --local context="$WORKSPACE" \
                            --local dockerfile="$WORKSPACE" \
                            --opt filename=Dockerfile \
                            --opt build-arg:VERSION="${BUILD_NUMBER}" \
                            --import-cache "type=registry,ref=${IMAGE_REPO}:buildcache" \
                            --export-cache "type=registry,ref=${IMAGE_REPO}:buildcache,mode=max" \
                            --output "type=image,name=${IMAGE_REPO}:${BUILD_NUMBER},push=true"
                        '''
                    }
                }
            }
        }

        stage('Deploy to Kubernetes') {
            steps {
                container('kubectl') {
                    sh '''
                    set -eu

                    TEMPLATE="$WORKSPACE/k8s/deployment.yaml.tpl"
                    RENDERED_MANIFEST="/tmp/go-cicd-demo-deployment-${BUILD_NUMBER}.yaml"

                    trap 'rm -f "$RENDERED_MANIFEST"' EXIT

                    echo "Rendering deployment image:"
                    echo "${IMAGE_REPO}:${BUILD_NUMBER}"

                    sed "s/__IMAGE_TAG__/${BUILD_NUMBER}/g" \
                        "$TEMPLATE" \
                        > "$RENDERED_MANIFEST"

                    if grep -q '__IMAGE_TAG__' "$RENDERED_MANIFEST"; then
                        echo "ERROR: image tag placeholder was not replaced"
                        exit 1
                    fi

                    echo "Rendered manifest image:"
                    grep 'image:' "$RENDERED_MANIFEST"

                    kubectl apply \
                        -f "$RENDERED_MANIFEST"

                    kubectl rollout status deployment/go-cicd-demo \
                        -n go-cicd-demo \
                        --timeout=180s

                    kubectl get deployment/go-cicd-demo \
                        -n go-cicd-demo \
                        -o wide
                    '''
                }
            }
        }

        stage('Smoke Test') {
            steps {
                container('kubectl') {
                    sh '''
                    set -eu

                    SERVICE_URL="http://go-cicd-demo.go-cicd-demo.svc.cluster.local"

                    for attempt in 1 2 3 4 5; do
                        ROOT_RESPONSE="$(
                            wget -qO- -T 10 "${SERVICE_URL}/" || true
                        )"

                        HEALTH_RESPONSE="$(
                            wget -qO- -T 10 "${SERVICE_URL}/healthz" || true
                        )"

                        VERSION_RESPONSE="$(
                            wget -qO- -T 10 "${SERVICE_URL}/version" || true
                        )"

                        echo "Smoke test attempt: ${attempt}"
                        echo "Root response: ${ROOT_RESPONSE}"
                        echo "Health response: ${HEALTH_RESPONSE}"
                        echo "Version response: ${VERSION_RESPONSE}"
                        echo "Expected version: ${BUILD_NUMBER}"

                        if [ "$ROOT_RESPONSE" = "Hello Kubernetes CI/CD" ] \
                            && [ "$HEALTH_RESPONSE" = "ok" ] \
                            && [ "$VERSION_RESPONSE" = "$BUILD_NUMBER" ]; then
                            echo "Smoke test passed"
                            exit 0
                        fi

                        echo "Smoke test attempt ${attempt} failed"
                        sleep 3
                    done

                    echo "Smoke test failed after 5 attempts"
                    exit 1
                    '''
                }
            }
        }
    }

    post {
        success {
            archiveArtifacts artifacts: 'app', fingerprint: true
        }
    }
}
