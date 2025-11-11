FROM ubuntu:22.04

# Avoid prompts from apt
ENV DEBIAN_FRONTEND=noninteractive

# Install dependencies
RUN apt-get update && apt-get install -y \
    nodejs \
    npm \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# Create orly user
RUN useradd -m -s /bin/bash orly

# Set working directory
WORKDIR /home/orly

# Copy pre-built binary (will be built on host)
COPY --chown=orly:orly orly /home/orly/.local/bin/orly

# Copy libsecp256k1.so for crypto operations
COPY --chown=orly:orly libsecp256k1.so /home/orly/.local/lib/libsecp256k1.so

# Copy policy files to the correct locations
COPY --chown=orly:orly cs-policy.js /home/orly/cs-policy.js
COPY --chown=orly:orly cs-policy-daemon.js /home/orly/cs-policy-daemon.js
COPY --chown=orly:orly policy.json /home/orly/.config/orly/policy.json
COPY --chown=orly:orly environment.txt /home/orly/env

# Create necessary directories (lowercase for config path)
RUN mkdir -p /home/orly/.config/orly && \
    mkdir -p /home/orly/.local/share/orly && \
    mkdir -p /home/orly/.local/bin && \
    mkdir -p /home/orly/.local/lib && \
    chown -R orly:orly /home/orly

# Switch to orly user
USER orly

# Set up environment
ENV PATH="/home/orly/.local/bin:${PATH}"
ENV LD_LIBRARY_PATH="/home/orly/.local/lib:${LD_LIBRARY_PATH}"

# Expose relay port
EXPOSE 8777

# Copy and set up the start script
COPY --chown=orly:orly start.sh /home/orly/start.sh

WORKDIR /home/orly

CMD ["/bin/bash", "/home/orly/start.sh"]
