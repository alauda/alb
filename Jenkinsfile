library "alauda-cicd"
def language = "golang"
AlaudaPipeline {
  config = [
    agent: 'golang-and-devops',
    folder: 'src/alb2',
    chart: [
      [
        enabled: false
      ]
    ],
    scm: [
      credentials: 'acp-acp-bitbucket-new'
    ],
    docker: [
      repository: "acp/alb2",
      credentials: "alaudak8s",
      context: ".",
      dockerfile: "Dockerfile.nginx.local",
    ],
    sonar: [
      binding: "sonarqube"
    ],
  ]
  env = [
    GO111MODULE: "on",
    GOPROXY: "https://goproxy.cn,direct",
  ]
  steps = [
    [
      name: "update chart"
      container: "devops"
      groovy:
      - '''
        def branchName = env.GIT_BRANCH
        def chartPath = "./chart"
        def isPullRequest = env.VERSION.contains("-pr-")
        def isMaster = branchName == "master"
        def isRelease = branchName ==~ /release-\d+\.\d+/
        def releaseType = "alpha"
        if (isRelease) {
          releaseType = "beta"
        } else if (isPullRequest || args.env == "int") {
          releaseType = "pr"
        }
        def generatedVersion = sh script: "kubectl devops chart version -n ${alaudaContext.getNamespace()} --name alb2 --version=${env.MAJOR_VERSION} --type=${releaseType}", label: "generating chart version", returnStdout: true
        sh: "kubectl devops chart update -v 3 --path ${chartPath} -c alb2=${env.VERSION} --annotations release=${releaseType},branch=${branchName} --chart-version=${generatedVersion}", label: "updating chart files"

        def harborAddress = "https://harbor-b.alauda.cn/chartrepo/acp"
        def harborCredentials = "cpaas-system-global-credentials-harbor-chart"
        container('devops') {
          withCredentials([usernamePassword(credentialsId: harborCredentials, passwordVariable: 'PASS', usernameVariable: 'USER')]) {
            sh script: "helm repo add harbor --username ${USER} --password ${PASS} ${harborAddress}", label: "helm repo add harbor"
          }
          sh script: "helm repo update", label: "helm repo update"
          sh script: "helm push ${chartPath} harbor", label: "helm push"
        }
        '''
    ]
  ]
}
