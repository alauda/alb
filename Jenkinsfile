@Library('alauda-cicd') _

// global variables for pipeline
def GIT_BRANCH = ""
def GIT_COMMIT = ""
def deployment
// image can be used for promoting...
def IMAGE
def CURRENT_VERSION = ""
def IMAGE_TAG = ""
def TEST_RESULT = true
def PROXY_CREDENTIALS

pipeline {
    agent {label 'golang-1.11'}
    options {
        buildDiscarder(logRotator(numToKeepStr: '10'))
        disableConcurrentBuilds()

        skipDefaultCheckout()
    }
    environment {
        FOLDER = 'src/alb2'
        // repository name
        REPOSITORY = "alb2"
        // repo user
        OWNER = "mathildetech"
        // context folder used for tooling
        CONTEXT = "."
        // for some debugging
        DEBUG = false
        // sonar feedback user
        BITBUCKET_FEEDBACK_ACCOUNT = "alaudabot"
        SONARQUBE_BITBUCKET_CREDENTIALS = "alaudabot"
        NAMESPACE = "alauda-system"
        DEPLOYMENT = "alb2"
        CONTAINER = "alb2"

        PROXY_CREDENTIALS_ID = 'proxy'
        TAG_CREDENTIALS = "alaudabot-bitbucket"
        GOPATH = "${WORKSPACE}"
    }
    stages {
        stage('Checkout') {
            steps {
                script {
                    dir(FOLDER) {
                        container('tools') {
                          // checkout code
                          withCredentials([usernamePassword(credentialsId: PROXY_CREDENTIALS_ID, passwordVariable: 'PROXY_ADDRESS', usernameVariable: 'PROXY_ADDRESS_PASS')]) {
                              PROXY_CREDENTIALS = "${PROXY_ADDRESS}"
                          }
                          sh "git config --global http.proxy ${PROXY_CREDENTIALS}"
                          def scmVars
                          retry(2) {
                              scmVars = checkout scm
                          }

                          // extract git information
                          env.GIT_COMMIT = scmVars.GIT_COMMIT
                          env.GIT_BRANCH = scmVars.GIT_BRANCH
                          CHANGE_TARGET = "${env.CHANGE_TARGET}"
                          CHANGE_TITLE = "${env.CHANGE_TITLE}"
                          GIT_COMMIT = "${scmVars.GIT_COMMIT}"
                          GIT_BRANCH = "${scmVars.GIT_BRANCH}"
                          RELEASE_BUILD = "${env.BUILD_NUMBER}"
                      }
                    }
                }
            }
        }

        stage('CI') {
            parallel {
                stage("Lint") {
                    steps {
                        script {
                            dir(FOLDER) {
                                container('golang') {
                                    sh "make lint"
                                }
                            }
                        }
                    }
                }
                stage("Unit Test") {
                    steps {
                        script {
                            dir(FOLDER) {
                                container('golang') {
                                    sh "make test"
                                }
                            }
                        }
                    }
                }
            }
        }

        stage('Code scan') {
            steps {
                echo "${REPOSITORY}"
                echo "${GIT_BRANCH}"
                echo "${CONTEXT}"
                echo "${BITBUCKET_FEEDBACK_ACCOUNT}"
                script {
                    dir(FOLDER) {
                        deploy.scan(
                            "${REPOSITORY}", 
                            "$GIT_BRANCH", 
                            "${SONARQUBE_BITBUCKET_CREDENTIALS}",
                            "${CONTEXT}",
                            false, 
                            "mathildetech", 
                            "${BITBUCKET_FEEDBACK_ACCOUNT}").start()
                    }
                }
            }
        }
    }
}
