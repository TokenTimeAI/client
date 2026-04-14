# syntax=docker/dockerfile:1
# check=error=true

ARG RUBY_VERSION=3.4.7
FROM docker.io/library/ruby:$RUBY_VERSION-slim AS base

WORKDIR /rails

# Install base packages - cached layer, only changes if packages change
# Using --mount for stricter caching on BuildKit
RUN --mount=type=cache,target=/var/cache/apt,sharing=locked \
    --mount=type=cache,target=/var/lib/apt,sharing=locked \
    apt-get update -qq && \
    apt-get install --no-install-recommends -y curl libjemalloc2 libvips postgresql-client && \
    ln -s /usr/lib/$(uname -m)-linux-gnu/libjemalloc.so.2 /usr/local/lib/libjemalloc.so && \
    rm -rf /var/lib/apt/lists /var/cache/apt/archives

ENV RAILS_ENV="production" \
    BUNDLE_DEPLOYMENT="1" \
    BUNDLE_PATH="/usr/local/bundle" \
    BUNDLE_WITHOUT="development" \
    LD_PRELOAD="/usr/local/lib/libjemalloc.so"

# Build stage
FROM base AS build

# Install build packages - cached layer
RUN --mount=type=cache,target=/var/cache/apt,sharing=locked \
    --mount=type=cache,target=/var/lib/apt,sharing=locked \
    apt-get update -qq && \
    apt-get install --no-install-recommends -y build-essential git libpq-dev libvips libyaml-dev pkg-config && \
    rm -rf /var/lib/apt/lists /var/cache/apt/archives

# Install gems - cached when Gemfile/Gemfile.lock unchanged
# Use cache mounts for bundler to persist gem downloads
RUN --mount=type=cache,target=/usr/local/bundle/cache,sharing=locked \
    --mount=type=bind,source=Gemfile,target=Gemfile \
    --mount=type=bind,source=Gemfile.lock,target=Gemfile.lock \
    bundle install && \
    bundle exec bootsnap precompile -j 1 --gemfile

# Copy app code - this layer changes most frequently
COPY . .

# Precompile bootsnap - cached when app/ lib/ unchanged
RUN bundle exec bootsnap precompile -j 1 app/ lib/

# Precompile assets - cached when assets unchanged
# Using dummy secret key since assets don't need real secrets
RUN SECRET_KEY_BASE_DUMMY=1 ./bin/rails assets:precompile

# Final stage
FROM base

RUN groupadd --system --gid 1000 rails && \
    useradd rails --uid 1000 --gid 1000 --create-home --shell /bin/bash
USER 1000:1000

# Copy only what's needed from build stage
COPY --chown=rails:rails --from=build "${BUNDLE_PATH}" "${BUNDLE_PATH}"
COPY --chown=rails:rails --from=build /rails /rails

ENTRYPOINT ["/rails/bin/docker-entrypoint"]
EXPOSE 80
CMD ["./bin/thrust", "./bin/rails", "server"]
