pipeline {

    agent {
        kubernetes {
            yaml '''
apiVersion: v1
kind: Pod
spec:
  containers:

  - name: golang
    image: golang:1.22
    command:
    - sleep
    args:
    - 99d

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
      value: "127.0.0.1,localhost,10.42.0.0/16,10.43.0.0/16,.svc,.cluster.local"
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

  volumes:
  - name: buildkit-cache
    emptyDir: {}
'''
        }
    }

    environment {
        IMAGE_REPO = 'ghcr.io/suellenmorrissey2461986jxe-maker/go-cicd-demo'
    }

    stages {

        stage('Go Test') {
            steps {
                container('golang') {
                    sh '''
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
                    git config --global --add safe.directory "$WORKSPACE"
                    go build -o app
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
                            --output "type=image,name=${IMAGE_REPO}:${BUILD_NUMBER},push=true"
                        '''
                    }
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
