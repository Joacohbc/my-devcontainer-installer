##
## BUN SETUP
##

# Install Bun for devuser
RUN su - devuser -c "curl -fsSL https://bun.sh/install | bash" && \
    su - devuser -c 'echo "export BUN_INSTALL=\"\$HOME/.bun\"" >> /home/devuser/.profile' && \
    su - devuser -c 'echo "export PATH=\"\$BUN_INSTALL/bin:\$PATH\"" >> /home/devuser/.profile'
