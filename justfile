# pushes all changes to the main branch
push +COMMIT_MESSAGE:
  git add .
  git commit -m "{{COMMIT_MESSAGE}}"
  git pull origin main
  git push origin main

tag +TAG_NAME:
  git tag {{TAG_NAME}}
  git push origin {{TAG_NAME}}

test:
  go test -race ./...

air:
  air -c .air.toml

build-ui:
  cd ui && bun install && npx quasar build

build:
  go build -o ./tmp/server ./cmd/server

build-cli:
  go build -o ./tmp/rh ./cmd/rh

install:
  go build -o $(go env GOPATH)/bin/rh ./cmd/rh
