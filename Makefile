
SPACE_HOST ?= www1.d3

.PHONY: ground space

all: ground space

ground:
	cd lsa && go install -v -ldflags "-X main.Version=$$(date +%Y-%m-%dT%H:%M:%S%z)"

space:
	rsync -avz --delete --exclude-from=Makefile_space_exclude ./ ${SPACE_HOST}:lsa/
# 	ssh ${SPACE_HOST} 'cd lsa/lsa-space && env GOROOT=$$HOME/go $$HOME/go/bin/go install -v'
	ssh ${SPACE_HOST} 'cd lsa/lsa-space && env go install -v'

pull_space:
	scp ${SPACE_HOST}:go/bin/lsa-space space-pulled

install_space:
	scp space-pulled ${SPACE_HOST}:bin/lsa-space
