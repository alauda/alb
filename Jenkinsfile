library "alauda-cicd"
def language = "golang"
AlaudaPipeline {
    config = [
    agent: 'golang-1.12',
    folder: 'src/alb2',
    chart: [
    chart: "alauda-alb2",
    component: "alauda-alb2",
    ],
    scm: [
    credentials: 'acp-acp-bitbucket-new'
    ],
    docker: [
    repository: "claas/alb2",
    credentials: "alaudak8s",
    context: ".",
    dockerfile: "Dockerfile.nginx.local",
    ],
    sonar: [
    binding: "sonarqube"
    ],
    ]
    env = [
    // GOPATH: env.WORKSPACE,
    GOPROXY: "https://athens.acp.alauda.cn",
    ]
    steps = [
    ]
}
