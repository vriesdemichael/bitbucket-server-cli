# Stage 1: build libfaketime with raw-syscall interception so the JVM's
# vDSO-routed clock_gettime calls are intercepted on native Linux kernels.
FROM ubuntu:24.04 AS faketime-builder

RUN apt-get update && apt-get install -y --no-install-recommends wget build-essential \
    && wget -qO /tmp/libfaketime.tar.gz \
        https://github.com/wolfcw/libfaketime/archive/refs/tags/v0.9.12.tar.gz \
    && tar -xzf /tmp/libfaketime.tar.gz -C /tmp \
    && cd /tmp/libfaketime-0.9.12/src \
    && FAKETIME_COMPILE_CFLAGS="-DFAKE_CLOCK_GETTIME_SYSCALL" make

# Stage 2: final image — only the compiled .so is added, no build tooling.
FROM atlassian/bitbucket:9.4.16

RUN mkdir -p /usr/lib/x86_64-linux-gnu/faketime
COPY --from=faketime-builder \
    /tmp/libfaketime-0.9.12/src/libfaketime.so.1 \
    /usr/lib/x86_64-linux-gnu/faketime/libfaketime.so.1