#!/bin/bash

# Asegúrate de que el servicio SSH esté configurado correctamente
if [ ! -d "/var/run/sshd" ]; then
    mkdir /var/run/sshd
fi

# Checkear si el usuario devuser ya existe
if [ $(id -u devuser 2>/dev/null || echo -1) -ge 0 ]; then
    echo "devuser password: $(cat /home/devuser/initial_password.txt)"
else
    # Crea el usuario devuser con la contraseña devuser y le da permisos de sudo
    useradd -m -s /bin/bash devuser
    echo 'devuser:devpass' | chpasswd
    usermod -aG sudo devuser
    
    # To disable Docker-outside-Docker, comment out the next line
    usermod -aG docker devuser # Add devuser to docker group
    echo "devpass" > /home/devuser/initial_password.txt
    echo "devuser password: $(cat /home/devuser/initial_password.txt)"
fi

# Checkear si root ya tiene una contraseña
if [ -f /root/initial_password.txt ]; then
    echo "initial root password: $(cat /root/initial_password.txt)"
else
    # Establece una contraseña para el usuario root
    INITIAL_PASSWORD=$(pwgen -s 32)
    echo $INITIAL_PASSWORD > /root/initial_password.txt
    echo "root:$INITIAL_PASSWORD" | chpasswd
    echo "initial root password: $INITIAL_PASSWORD"
fi

# Inicia el servicio SSH
/usr/sbin/sshd -D -o ListenAddress=0.0.0.0