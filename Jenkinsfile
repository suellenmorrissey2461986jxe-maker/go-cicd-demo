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
