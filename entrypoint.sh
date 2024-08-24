#!/bin/bash

# Asegúrate de que el servicio SSH esté configurado correctamente
if [ ! -d "/var/run/sshd" ]; then
    mkdir /var/run/sshd
fi

# Inicia el servicio SSH
/usr/sbin/sshd -D -o ListenAddress=0.0.0.0