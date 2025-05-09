# Utiliza una imagen base de Ubuntu
FROM ubuntu:22.04

# Actualiza los paquetes e instala SSH, sudo, y otras utilidades
RUN apt-get update && apt-get install -y \
    openssh-server \
    nano \
    sudo \
    pwgen \
    # To disable Docker-outside-Docker, comment out ca-certificates, curl, gnupg, lsb-release if they are not needed by other packages
    ca-certificates \
    curl \
    gnupg \
    lsb-release \
    && apt-get clean

# To disable Docker-outside-Docker, comment out the following lines for Docker GPG key and repository setup
# Add Docker's official GPG key
RUN mkdir -p /etc/apt/keyrings
RUN curl -fsSL https://download.docker.com/linux/ubuntu/gpg | gpg --dearmor -o /etc/apt/keyrings/docker.gpg

# Set up the repository
RUN echo \
    "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/ubuntu \
    $(lsb_release -cs) stable" | tee /etc/apt/sources.list.d/docker.list > /dev/null

# To disable Docker-outside-Docker, comment out the next line
# Install Docker CLI
RUN apt-get update && apt-get install -y docker-ce-cli && apt-get clean

# Configura el servicio SSH
RUN mkdir /var/run/sshd && \
    chmod 755 /var/run/sshd # Asegura permisos correctos

# Configura SSH para permitir el inicio de sesión de root y el uso de contraseñas
RUN sed -i 's/^#PermitRootLogin prohibit-password/PermitRootLogin yes/' /etc/ssh/sshd_config
RUN sed -i 's/^#PasswordAuthentication yes/PasswordAuthentication yes/' /etc/ssh/sshd_config

# Expone el puerto 22 para SSH
EXPOSE 22

# Copia el script de entrypoint
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

# Establece el script de entrada
ENTRYPOINT ["/entrypoint.sh"]
