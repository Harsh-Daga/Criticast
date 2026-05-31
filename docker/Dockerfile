# Dev image for BPF build and benchmarks (Linux, privileged at runtime).
FROM ubuntu:24.04

ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update && apt-get install -y --no-install-recommends \
    bpftrace \
    clang \
    llvm \
    llvm-dev \
    libelf-dev \
    libbpf-dev \
    linux-tools-common \
    make \
    git \
    curl \
    ca-certificates \
    wrk \
    && rm -rf /var/lib/apt/lists/* \
    && if ! command -v bpftool >/dev/null 2>&1; then \
      tool="$(find /usr/lib/linux-tools -name bpftool -type f 2>/dev/null | head -1)"; \
      test -n "$tool" && ln -sf "$tool" /usr/local/bin/bpftool; \
    fi \
    && command -v bpftool

# Go 1.22+ (distro golang may lag)
ARG GO_VERSION=1.22.10
RUN curl -fsSL "https://go.dev/dl/go${GO_VERSION}.linux-$(dpkg --print-architecture).tar.gz" \
    | tar -C /usr/local -xz
ENV PATH="/usr/local/go/bin:${PATH}"

WORKDIR /src
COPY scripts/check-linux-env.sh /usr/local/bin/criticast-check-env
RUN chmod +x /usr/local/bin/criticast-check-env

# Default: interactive shell; override in compose.
CMD ["bash"]
