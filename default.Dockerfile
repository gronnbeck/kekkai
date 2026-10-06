FROM debian:bookworm-slim

RUN apt-get update \
 && apt-get install -y --no-install-recommends \
      bash ca-certificates curl git jq less openssh-client procps ripgrep unzip \
 && rm -rf /var/lib/apt/lists/* \
 && git config --system safe.directory '*'

RUN useradd --create-home --uid 1000 --shell /bin/bash claude
USER claude
ENV HOME=/home/claude \
    PATH=/home/claude/.local/bin:$PATH \
    DISABLE_AUTOUPDATER=1

RUN curl -fsSL https://claude.ai/install.sh | bash
