FROM debian:bookworm-slim

ARG GO_VERSION=1.26.5
ARG TARGETARCH

RUN apt-get update \
 && apt-get install -y --no-install-recommends \
      bash build-essential ca-certificates curl git jq less openssh-client procps ripgrep shellcheck unzip \
 && rm -rf /var/lib/apt/lists/* \
 && git config --system safe.directory '*'

RUN curl -fsSL "https://go.dev/dl/go${GO_VERSION}.linux-${TARGETARCH:-arm64}.tar.gz" | tar -C /usr/local -xz

RUN useradd --create-home --uid 1000 --shell /bin/bash claude
USER claude
# Module and build caches live in HOME so Linux builds stay out of the mounted repo
ENV HOME=/home/claude \
    GOPATH=/home/claude/go \
    GOCACHE=/home/claude/.cache/go-build \
    GOTOOLCHAIN=local \
    PATH=/home/claude/.local/bin:/home/claude/go/bin:/usr/local/go/bin:$PATH \
    DISABLE_AUTOUPDATER=1

RUN curl -fsSL https://claude.ai/install.sh | bash
