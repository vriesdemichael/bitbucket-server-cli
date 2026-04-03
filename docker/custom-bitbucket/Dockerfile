# Stage 1: build libfaketime with raw-syscall interception so the JVM's
# vDSO-routed clock_gettime calls are intercepted on native Linux kernels.
# The tarball is vendored (docker/custom-bitbucket/libfaketime-0.9.12.tar.gz)
# to avoid any network dependency during the CI Docker build.
# sha256: 4fc32218697c052adcdc5ee395581f2554ca56d086ac817ced2be0d6f1f8a9fa
FROM ubuntu:24.04 AS faketime-builder

COPY libfaketime-0.9.12.tar.gz /tmp/libfaketime.tar.gz
RUN apt-get update && apt-get install -y --no-install-recommends build-essential \
    && tar -xzf /tmp/libfaketime.tar.gz -C /tmp \
    && cd /tmp/libfaketime-0.9.12/src \
    && FAKETIME_COMPILE_CFLAGS="-DFAKE_CLOCK_GETTIME_SYSCALL" make

# Stage 2: final image — only the compiled .so is added, no build tooling.
FROM atlassian/bitbucket:9.4.16

RUN mkdir -p /usr/lib/x86_64-linux-gnu/faketime
COPY --from=faketime-builder \
    /tmp/libfaketime-0.9.12/src/libfaketime.so.1 \
    /usr/lib/x86_64-linux-gnu/faketime/libfaketime.so.1