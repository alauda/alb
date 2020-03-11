module alauda.io/alb2

go 1.14

require (
	github.com/evanphx/json-patch v4.5.0+incompatible
	github.com/fatih/set v0.2.1
	github.com/golang/glog v0.0.0-20160126235308-23def4e6c14b
	github.com/golang/groupcache v0.0.0-20181024230925-c65c006176ff // indirect
	github.com/google/go-cmp v0.3.1 // indirect
	github.com/googleapis/gnostic v0.2.0 // indirect
	github.com/spf13/cast v1.3.1
	github.com/spf13/viper v1.3.2
	github.com/stretchr/testify v1.3.0
	github.com/thoas/go-funk v0.0.0-20181015191849-9132db0aefe2
	golang.org/x/oauth2 v0.0.0-20191202225959-858c2ad4c8b6 // indirect
	google.golang.org/appengine v1.6.5 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	k8s.io/api v0.0.0-20191004120003-3a12735a829a
	k8s.io/apiextensions-apiserver v0.0.0-20191014073952-6b6acca910e3
	k8s.io/apimachinery v0.15.10-beta.0
	k8s.io/client-go v0.0.0-20191004120415-b2f42092e376
	k8s.io/utils v0.0.0-20191114200735-6ca3b61696b6 // indirect
)

replace k8s.io/api => k8s.io/api v0.0.0-20190918155943-95b840bb6a1f

replace k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.0.0-20190918161926-8f644eb6e783

replace k8s.io/apimachinery => k8s.io/apimachinery v0.0.0-20190913080033-27d36303b655

replace k8s.io/apiserver => k8s.io/apiserver v0.0.0-20190918160949-bfa5e2e684ad

replace k8s.io/cli-runtime => k8s.io/cli-runtime v0.0.0-20190918162238-f783a3654da8

replace k8s.io/client-go => k8s.io/client-go v0.0.0-20190918160344-1fbdaa4c8d90
