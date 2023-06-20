FROM golang:1.18  AS build-env
RUN apt update
RUN apt install git gcc musl-dev make -y
RUN go install github.com/google/wire/cmd/wire@latest
WORKDIR /go/src/github.com/devtron-labs/central-api
ADD . /go/src/github.com/devtron-labs/central-api
RUN GOOS=linux make

FROM alpine:3.9
RUN apk add --no-cache ca-certificates
COPY --from=build-env  /go/src/github.com/devtron-labs/central-api/central-api .
COPY ./DockerfileTemplateData.json /DockerfileTemplateData.json
COPY ./BuildpackMetadata.json /BuildpackMetadata.json
CMD ["./central-api"]