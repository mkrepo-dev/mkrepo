FROM golang:1.24 AS build

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

ARG MODULE
ARG VERSION
ARG DATETIME
ARG REVISION

COPY ./ ./
RUN CGO_ENABLED=0 GOAMD64=v2 go build \
    -ldflags "-X $MODULE/internal.version=$VERSION -X $MODULE/internal.revision=$REVISION -X $MODULE/internal.buildDatetime=$DATETIME" \
    -o bin/server $MODULE/cmd/server


FROM gcr.io/distroless/static-debian12

WORKDIR /app

COPY --from=build /app/bin/ .

CMD ["./server"]

ARG VERSION
ARG DATETIME
ARG REVISION
ARG IMAGE_REF

LABEL \
    org.opencontainers.image.title="mkrepo" \
    org.opencontainers.image.description="mkrepo is webapp for bootstraping repo on diffrent VCS providers" \
    org.opencontainers.image.version="$VERSION" \
    org.opencontainers.image.created="$DATETIME" \
    org.opencontainers.image.authors="Filip Solich" \
    org.opencontainers.image.licenses="Apache-2.0 license" \
    org.opencontainers.image.url="mkrepo.io" \
    org.opencontainers.image.documentation="https://github.com/FilipSolich/mkrepo/blob/main/README.md" \
    org.opencontainers.image.source="https://github.com/FilipSolich/mkrepo" \
    org.opencontainers.image.revision="$REVISION" \
    org.opencontainers.image.ref.name="$IMAGE_REF" \
    org.opencontainers.image.base.name="gcr.io/distroless/static-debian12"
