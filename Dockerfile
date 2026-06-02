FROM golang:1.26 AS build

WORKDIR /app

COPY ./ ./
RUN scripts/build.sh


FROM gcr.io/distroless/static-debian13

WORKDIR /

COPY --from=build /app/bin/mkrepo .

CMD ["./mkrepo", "server"]

ARG VERSION BUILD_DATETIME REVISION IMAGE_REF

LABEL \
    org.opencontainers.image.title="mkrepo" \
    org.opencontainers.image.description="mkrepo is tool for bootstraping git repo on diffrent VCS providers" \
    org.opencontainers.image.version="$VERSION" \
    org.opencontainers.image.created="$BUILD_DATETIME" \
    org.opencontainers.image.authors="Filip Solich" \
    org.opencontainers.image.licenses="MIT License" \
    org.opencontainers.image.url="https://mkrepo.dev" \
    org.opencontainers.image.documentation="https://github.com/mkrepo-dev/mkrepo/blob/main/README.md" \
    org.opencontainers.image.source="https://github.com/mkrepo-dev/mkrepo" \
    org.opencontainers.image.revision="$REVISION" \
    org.opencontainers.image.ref.name="$IMAGE_REF" \
    org.opencontainers.image.base.name="gcr.io/distroless/static-debian13"
