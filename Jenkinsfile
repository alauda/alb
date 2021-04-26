library "alauda-cicd"

def language = "golang"

AlaudaPipeline{
    config = [
        agent: 'golang-1.13',
        folder: '.',
        chart: [
            pipeline: "chart-alb2",
            project: "acp",
            chart: "alauda-alb2",
            component: "alb2",
        ],
        scm: [
            credentials: 'cpaas-system-global-credentials-acp-alauda-gitlab'
        ],
        docker: [
            repository: "acp/alb2",
            credentials: "alaudak8s",
            context: ".",
            dockerfile: "Dockerfile.nginx.local",
            disableArmBuildOnPR: true
        ],
        sonar: [
            binding: "sonarqube"
        ],
    ]
    env = [
        GOPATH: "",
        
        GOPROXY: "https://athens.alauda.cn",
    ]
    yaml = "alauda.yaml"
}
