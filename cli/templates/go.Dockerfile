# Install Go
COPY golang_utils.sh /tmp/golang_utils.sh
RUN bash -c "source /tmp/golang_utils.sh && install_golang" && rm /tmp/golang_utils.sh
