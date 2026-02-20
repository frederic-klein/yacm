FROM debian:12-slim

# Suppress noisy macOS extended attribute pax header warnings
ENV TAR_OPTIONS="--warning=no-unknown-keyword"

# Install minimal build dependencies (no pkg-config to match Carmel behavior)
RUN apt-get update && apt-get install -y --no-install-recommends \
    build-essential \
    ca-certificates \
    curl \
    libpq-dev \
    libssl-dev \
    libxml2-dev \
    perl \
    zlib1g-dev \
    && rm -rf /var/lib/apt/lists/*

# Install cpanminus and required Perl modules for running Makefile.PL
RUN curl -L https://cpanmin.us | perl - App::cpanminus \
    && cpanm --notest \
        File::Which \
        ExtUtils::CBuilder \
        ExtUtils::ParseXS \
        JSON::PP \
        Path::Tiny \
        Capture::Tiny \
    && rm -rf ~/.cpanm

WORKDIR /work
