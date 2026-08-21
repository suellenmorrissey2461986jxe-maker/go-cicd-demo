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
'''
        }
    }


    stages {


        stage('Checkout') {

            steps {

                git branch: 'main',
                    url: 'git@github.com:suellenmorrissey2461986jxe-maker/go-cicd-demo.git'

            }

        }


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
                    go build -o app
                    ls -lh app
                    '''

                }

            }

        }

    }
}
