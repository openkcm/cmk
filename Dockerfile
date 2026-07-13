# Use distroless as minimal base image to package the component binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM gcr.io/distroless/static-debian12:nonroot@sha256:b7bb25d9f7c31d2bdd1982feb4dafcaf137703c7075dbe2febb41c24212b946f
ARG TARGETOS
ARG TARGETARCH
ARG COMPONENT
WORKDIR /
COPY bin/$COMPONENT.$TARGETOS-$TARGETARCH /<component>
USER 65532:65532

# docker doesn't substitue args in ENTRYPOINT, so we replace this during the build script
ENTRYPOINT ["/<component>"]
