FROM golang:1.24 AS build

WORKDIR /app

RUN curl -L -o /usr/local/bin/atlas https://release.ariga.io/atlas/atlas-linux-amd64-latest &&\
    chmod +x /usr/local/bin/atlas

RUN go install github.com/go-task/task/v3/cmd/task@latest

COPY go.mod go.sum ./
RUN go mod download

COPY ./ ./
RUN task build


FROM gcr.io/distroless/static-debian12

WORKDIR /app

COPY --from=build /usr/local/bin/atlas /usr/local/bin/atlas
COPY --from=build /app/bin/ .
COPY --from=build /app/LICENSE .

CMD ["./server"]

ARG VERSION
ARG DATETIME
ARG REVISION
ARG IMAGE_REF

LABEL \
    org.opencontainers.image.title="mkrepo" \
    org.opencontainers.image.description="mkrepo is tool for bootstraping git repo on diffrent VCS providers" \
    org.opencontainers.image.version="$VERSION" \
    org.opencontainers.image.created="$DATETIME" \
    org.opencontainers.image.authors="Filip Solich" \
    org.opencontainers.image.licenses="MIT License" \
    org.opencontainers.image.url="https://mkrepo.dev" \
    org.opencontainers.image.documentation="https://github.com/mkrepo-dev/mkrepo/blob/main/README.md" \
    org.opencontainers.image.source="https://github.com/mkrepo-dev/mkrepo" \
    org.opencontainers.image.revision="$REVISION" \
    org.opencontainers.image.ref.name="$IMAGE_REF" \
    org.opencontainers.image.base.name="gcr.io/distroless/static-debian12"
