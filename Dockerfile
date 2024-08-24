# Utiliza una imagen base de Ubuntu
FROM ubuntu:22.04

# Actualiza los paquetes e instala SSH y sudo
RUN apt-get update && apt-get install -y \
    openssh-server \
    sudo \
    && apt-get clean

# Configura el servicio SSH
RUN mkdir /var/run/sshd && \
    chmod 755 /var/run/sshd # Asegura permisos correctos

# Configura SSH para permitir el inicio de sesión de root y el uso de contraseñas
RUN sed -i 's/^#PermitRootLogin prohibit-password/PermitRootLogin yes/' /etc/ssh/sshd_config
RUN sed -i 's/^#PasswordAuthentication yes/PasswordAuthentication yes/' /etc/ssh/sshd_config

# Crea el usuario devuser con la contraseña devuser y le da permisos de sudo
RUN useradd -m -s /bin/bash devuser \
    && echo 'devuser:devuser' | chpasswd \
    && usermod -aG sudo devuser

# Establece una contraseña para el usuario root (opcional)
RUN echo 'root:rootpassword' | chpasswd



# Expone el puerto 22 para SSH
EXPOSE 22

# Copia el script de entrypoint
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

# Establece el script de entrada
ENTRYPOINT ["/entrypoint.sh"]