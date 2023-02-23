UNAME:=$(shell uname)

ifeq ($(UNAME),Linux)
	SED = sed
endif
ifeq ($(UNAME),Darwin)
	SED = gsed
endif

.PHONY: test

gen-crd-and-client: 
	bash -c "source ./scripts/alb-deploy-actions.sh ; alb-gen-crd-and-client"

static-build: 
	bash -c "source ./scripts/alb-build-actions.sh ; alb-static-build"