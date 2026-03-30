##
## CLEANUP & ENTRYPOINT
##

# Clean up
RUN apt-get autoremove -y && \
    apt-get autoclean && \
    rm -rf /var/lib/apt/lists/* && \
    rm -rf /tmp/* && \
    rm -rf /var/tmp/*

# Configure the SSH service
RUN mkdir /var/run/sshd && \
    chmod 755 /var/run/sshd

# Expose port 22 for SSH
EXPOSE 22

# Copy the entrypoint script
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

# Set the entrypoint script
ENTRYPOINT ["/entrypoint.sh"]