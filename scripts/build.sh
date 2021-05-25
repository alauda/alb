set -o errexit
export CGO_ENABLED=0 
go build -ldflags "-w -s" -v -o ./alb alauda.io/alb2
go build -ldflags "-w -s" -v -o ./migrate_v26tov28 alauda.io/alb2/migrate/v26tov28
go build -ldflags "-w -s" -v -o ./migrate_priority alauda.io/alb2/migrate/priority
md5sum ./alb