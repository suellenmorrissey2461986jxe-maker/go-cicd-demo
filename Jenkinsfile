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
    image: golang:1.26.7
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

    parameters {
        booleanParam(
            name: 'DEPLOY_AFTER_BUILD',
            defaultValue: false,
            description: 'Deploy only after the candidate image has passed vulnerability review'
        )
        booleanParam(
            name: 'FORCE_SMOKE_FAILURE',
            defaultValue: false,
            description: 'Intentionally fail the smoke test to verify automatic rollback'
        )
    }

    environment {
        IMAGE_REPO = '100.113.248.106:30002/go-cicd-demo/go-cicd-demo'
        GITOPS_REPO = 'git@github.com:suellenmorrissey2461986jxe-maker/go-cicd-demo-gitops.git'
        GITOPS_BRANCH = 'main'
    }

    stages {

        stage('Checkout') {
            steps {
                script {
                    env.SCAN_GATE_PASSED = 'false'
                    env.IMAGE_DIGEST = ''
                    env.DEPLOY_IMAGE = ''
                    sh 'rm -f .deployment-started .previous-deployment-image .image-digest .build-metadata.json .harbor-gate'
                    def scmVars

                    retry(3) {
                        scmVars = checkout scm
                    }

                    env.IMAGE_TAG = "git-${scmVars.GIT_COMMIT.take(12)}"

                    echo "Git commit: ${scmVars.GIT_COMMIT}"
                    echo "Image tag: ${env.IMAGE_TAG}"
                    echo "Branch name: ${env.BRANCH_NAME ?: 'single-pipeline'}"
                }
            }
        }

        stage('Validate GitOps Credential') {
            when {
                expression {
                    return params.DEPLOY_AFTER_BUILD == true &&
                        (!env.BRANCH_NAME || env.BRANCH_NAME == 'main')
                }
            }

            steps {
                withCredentials([
                    string(
                        credentialsId: 'github-gitops-ssh-v3-b64',
                        variable: 'GITOPS_SSH_KEY_B64'
                    )
                ]) {
                    sh '''
                    set -eu
                    set +x

                    TEST_KEY="$WORKSPACE/.gitops-key-test"
                    trap 'rm -f "$TEST_KEY"' EXIT

                    printf '%s' "$GITOPS_SSH_KEY_B64" | tr -d '[:space:]' |
                        base64 -d > "$TEST_KEY"
                    chmod 600 "$TEST_KEY"

                    ssh-keygen -y -f "$TEST_KEY" >/dev/null
                    echo "GitOps SSH private key format validated"

                    export GIT_SSH_COMMAND="ssh -F /dev/null -i $TEST_KEY -o IdentitiesOnly=yes -o IdentityAgent=none -o BatchMode=yes -o StrictHostKeyChecking=accept-new -o ConnectTimeout=15"
                    git ls-remote \
                        "$GITOPS_REPO" \
                        "refs/heads/$GITOPS_BRANCH"
                    echo "GitOps repository SSH access validated"
                    '''
                }
            }
        }

        stage('Go Test') {
            steps {
                container('golang') {
                    sh '''
                    set -eu

                    go version
                    GOMAXPROCS=2 go test -p 2 ./...
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
                        -ldflags="-X main.version=${IMAGE_TAG}" \
                        -o app .

                    ls -lh app
                    '''
                }
            }
        }

        stage('Build and Push Image') {
            when {
                expression {
                    return !env.BRANCH_NAME || env.BRANCH_NAME == 'main'
                }
            }

            steps {
                container('buildkit') {
                    withCredentials([
                        usernamePassword(
                            credentialsId: 'harbor-credentials',
                            usernameVariable: 'HARBOR_USER',
                            passwordVariable: 'HARBOR_SECRET'
                        )
                    ]) {
                        sh '''
                        set -eu
                        set +x

                        export DOCKER_CONFIG=/home/user/.docker
                        mkdir -p "$DOCKER_CONFIG"

                        AUTH="$(printf '%s:%s' "$HARBOR_USER" "$HARBOR_SECRET" \
                            | base64 | tr -d '\n')"

                        printf '{"auths":{"100.113.248.106:30002":{"auth":"%s"}}}\n' "$AUTH" \
                            > "$DOCKER_CONFIG/config.json"

                        unset AUTH
                        trap 'rm -f "$DOCKER_CONFIG/config.json"' EXIT

                        set -x

                        buildctl-daemonless.sh build \
                            --metadata-file "$WORKSPACE/.build-metadata.json" \
                            --frontend dockerfile.v0 \
                            --local context="$WORKSPACE" \
                            --local dockerfile="$WORKSPACE" \
                            --opt filename=Dockerfile \
                            --opt build-arg:VERSION="${IMAGE_TAG}" \
                            --import-cache "type=registry,ref=${IMAGE_REPO}:buildcache,registry.insecure=true" \
                            --export-cache "type=registry,ref=${IMAGE_REPO}:buildcache,mode=max,registry.insecure=true" \
                            --output "type=image,name=${IMAGE_REPO}:${IMAGE_TAG},push=true,registry.insecure=true"
                        '''
                    }
                }
            }
        }

        stage('Harbor Scan Gate') {
            when {
                expression {
                    return !env.BRANCH_NAME || env.BRANCH_NAME == 'main'
                }
            }
            options {
                timeout(time: 12, unit: 'MINUTES')
            }
            steps {
                container('golang') {
                    sh '''
                    set -eu
                    GOMAXPROCS=2 go build -buildvcs=false -p 2 \
                        -o "$WORKSPACE/.harbor-gate" ./cmd/harbor-gate
                    '''
                    withCredentials([
                        usernamePassword(
                            credentialsId: 'harbor-scan-credentials',
                            usernameVariable: 'SCAN_USER',
                            passwordVariable: 'SCAN_SECRET'
                        )
                    ]) {
                        // A nonzero gate exit must fail the stage, including BLOCK=3.
                        sh '''
                        set -eu
                        "$WORKSPACE/.harbor-gate" scan \
                            "$WORKSPACE/.build-metadata.json" \
                            "$WORKSPACE/.image-digest"
                        '''
                    }
                }
                script {
                    env.IMAGE_DIGEST = readFile('.image-digest').trim()
                    if (!(env.IMAGE_DIGEST ==~ /sha256:[0-9a-f]{64}/)) {
                        error('Invalid digest returned by the scan gate')
                    }
                    env.DEPLOY_IMAGE = "${env.IMAGE_REPO}@${env.IMAGE_DIGEST}"
                    env.SCAN_GATE_PASSED = 'true'
                    echo "Approved deployment image: ${env.DEPLOY_IMAGE}"
                }
            }
        }

        stage('Update GitOps Repository') {
            when {
                expression {
                    return params.DEPLOY_AFTER_BUILD == true &&
                        (!env.BRANCH_NAME || env.BRANCH_NAME == 'main')
                }
            }

            steps {
                script {
                    if (env.SCAN_GATE_PASSED != 'true' ||
                        !(env.IMAGE_DIGEST ==~ /sha256:[0-9a-f]{64}/)) {
                        error('GitOps publication requires a successful scan gate')
                    }
                }
                withCredentials([
                    string(
                        credentialsId: 'github-gitops-ssh-v3-b64',
                        variable: 'GITOPS_SSH_KEY_B64'
                    )
                ]) {
                    retry(3) {
                        sh '''
                        set -eu
                        set +x

                        GITOPS_DIR="$WORKSPACE/.gitops-work"
                        rm -rf "$GITOPS_DIR"

                        mkdir -p "$HOME/.ssh"
                        chmod 700 "$HOME/.ssh"

                        NORMALIZED_GITOPS_KEY="$WORKSPACE/.gitops-key"
                        trap 'rm -f "$NORMALIZED_GITOPS_KEY"' EXIT

                        printf '%s' "$GITOPS_SSH_KEY_B64" | tr -d '[:space:]' |
                            base64 -d > "$NORMALIZED_GITOPS_KEY"
                        chmod 600 "$NORMALIZED_GITOPS_KEY"

                        if ! ssh-keygen -y -f "$NORMALIZED_GITOPS_KEY" >/dev/null 2>&1; then
                            echo "ERROR: Jenkins credential github-gitops-ssh-v3-b64 is not a valid OpenSSH private key"
                            exit 1
                        fi

                        echo "GitOps SSH private key format validated"
                        export GIT_SSH_COMMAND="ssh -F /dev/null -i $NORMALIZED_GITOPS_KEY -o IdentitiesOnly=yes -o IdentityAgent=none -o BatchMode=yes -o StrictHostKeyChecking=accept-new -o ConnectTimeout=15"

                        git clone \
                            --branch "$GITOPS_BRANCH" \
                            --single-branch \
                            "$GITOPS_REPO" \
                            "$GITOPS_DIR"

                        cd "$GITOPS_DIR"

                        git config user.name "Jenkins GitOps"
                        git config user.email "jenkins-gitops@local"

                        test "$(grep -cE '^[[:space:]]+image:' deployment.yaml)" -eq 1

                        sed -i -E \
                            "s|^([[:space:]]*image:[[:space:]]*).*$|\1${DEPLOY_IMAGE}|" \
                            deployment.yaml

                        grep -F "image: ${DEPLOY_IMAGE}" deployment.yaml
                        git diff --check
                        git diff -- deployment.yaml
                        git add deployment.yaml

                        if git diff --cached --quiet; then
                            echo "GitOps manifest already references ${DEPLOY_IMAGE}"
                        else
                            git commit -m "Deploy ${IMAGE_TAG} via Jenkins GitOps"
                            git push origin "HEAD:${GITOPS_BRANCH}"
                        fi

                        git rev-parse HEAD > "$WORKSPACE/.gitops-revision"
                        echo "Published GitOps revision: $(cat "$WORKSPACE/.gitops-revision")"
                        '''
                    }
                }
            }
        }

        stage('Deploy to Kubernetes') {
            when {
                expression {
                    return false
                }
            }

            steps {
                script {
                    if (env.SCAN_GATE_PASSED != 'true' ||
                        !(env.IMAGE_DIGEST ==~ /sha256:[0-9a-f]{64}/)) {
                        error('Deployment requires a successful scan gate')
                    }
                }
                container('kubectl') {
                    sh '''
                    set -eu

                    TEMPLATE="$WORKSPACE/k8s/deployment.yaml.tpl"
                    RENDERED_MANIFEST="/tmp/go-cicd-demo-deployment-${IMAGE_TAG}.yaml"
                    PREVIOUS_IMAGE_FILE="$WORKSPACE/.previous-deployment-image"
                    DEPLOYMENT_MARKER="$WORKSPACE/.deployment-started"

                    trap 'rm -f "$RENDERED_MANIFEST"' EXIT

                    rm -f \
                        "$PREVIOUS_IMAGE_FILE" \
                        "$DEPLOYMENT_MARKER"

                    PREVIOUS_IMAGE="$(
                        kubectl get deployment/go-cicd-demo \
                            -n go-cicd-demo \
                            -o jsonpath='{.spec.template.spec.containers[?(@.name=="app")].image}'
                    )"

                    if [ -z "$PREVIOUS_IMAGE" ]; then
                        echo "ERROR: unable to determine the current deployment image"
                        exit 1
                    fi

                    printf '%s\n' "$PREVIOUS_IMAGE" \
                        > "$PREVIOUS_IMAGE_FILE"

                    echo "Previous stable image: $PREVIOUS_IMAGE"
                    echo "Rendering deployment image:"
                    echo "${DEPLOY_IMAGE}"

                    sed "s|__IMAGE_REFERENCE__|${DEPLOY_IMAGE}|g" \
                        "$TEMPLATE" \
                        > "$RENDERED_MANIFEST"

                    if grep -q '__IMAGE_REFERENCE__' "$RENDERED_MANIFEST"; then
                        echo "ERROR: image reference placeholder was not replaced"
                        exit 1
                    fi

                    echo "Rendered manifest image:"
                    grep 'image:' "$RENDERED_MANIFEST"

                    kubectl apply \
                        -f "$RENDERED_MANIFEST"

                    touch "$DEPLOYMENT_MARKER"

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
            when {
                expression {
                    return false
                }
            }

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
                        echo "Expected version: ${IMAGE_TAG}"

                        if [ "${FORCE_SMOKE_FAILURE:-false}" = "true" ]; then
                            echo "Controlled smoke test failure requested"
                            exit 1
                        fi

                        if [ "$ROOT_RESPONSE" = "Hello Kubernetes CI/CD" ] \
                            && [ "$HEALTH_RESPONSE" = "ok" ] \
                            && [ "$VERSION_RESPONSE" = "$IMAGE_TAG" ]; then
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
        failure {
            script {
                if (
                    params.DEPLOY_AFTER_BUILD == true &&
                    fileExists('.deployment-started') &&
                    fileExists('.previous-deployment-image')
                ) {
                    container('kubectl') {
                        sh '''
                        set -eu

                        PREVIOUS_IMAGE="$(
                            cat "$WORKSPACE/.previous-deployment-image"
                        )"

                        echo "===== Automatic rollback started ====="
                        echo "Restoring image: $PREVIOUS_IMAGE"

                        kubectl rollout undo deployment/go-cicd-demo \
                            -n go-cicd-demo

                        kubectl rollout status deployment/go-cicd-demo \
                            -n go-cicd-demo \
                            --timeout=180s

                        ROLLED_BACK_IMAGE="$(
                            kubectl get deployment/go-cicd-demo \
                                -n go-cicd-demo \
                                -o jsonpath='{.spec.template.spec.containers[?(@.name=="app")].image}'
                        )"

                        echo "Expected rollback image: $PREVIOUS_IMAGE"
                        echo "Current deployment image: $ROLLED_BACK_IMAGE"

                        if [ "$ROLLED_BACK_IMAGE" != "$PREVIOUS_IMAGE" ]; then
                            echo "ERROR: deployment image was not restored"
                            exit 1
                        fi

                        kubectl get pods \
                            -n go-cicd-demo \
                            -o wide

                        echo "===== Automatic rollback completed ====="
                        '''
                    }
                } else {
                    echo 'Pipeline failed before deployment; rollback skipped'
                }
            }
        }

        success {
            archiveArtifacts artifacts: 'app', fingerprint: true
        }
    }
}
