#!/bin/bash

# Arquitectura a descargar
ARCH="linux-arm64"

apt-get install curl -y

install_golang() {
    TEMP_DIR=$(mktemp -d)
    if [[ ! -d "$TEMP_DIR" ]]; then
        echo "Error al crear el directorio temporal."
        return 1
    fi
    
    echo "Directorio temporal: $TEMP_DIR"

    # Obtiene el HTML de la página de descargas
    GO_RELEASE_PAGE=$(curl -sS https://go.dev/dl/ || { echo "Error al obtener la página de descargas."; exit 1; })

    # Extrae el nombre del archivo de la última versión
    GO_LATEST_VERSION=$(echo "$GO_RELEASE_PAGE" | grep -oE "go[0-9.]+\.[a-z0-9-]+\.tar\.gz" | grep $ARCH | head -n 1)

    # Verifica si se encontró un enlace
    DEFAULT_GO_VERSION="go1.20.linux-arm64.tar.gz" #Versión por defecto
    if [[ -z "$GO_LATEST_VERSION" ]]; then
        echo "No se encontró el enlace de descarga para $ARCH. Se instalará la versión por defecto."
        GO_LATEST_VERSION=$DEFAULT_GO_VERSION
    fi

    # Construye la URL completa
    GO_DOWNLOAD_URL="https://go.dev/dl/$GO_LATEST_VERSION"
    echo "URL de descarga: $GO_DOWNLOAD_URL"

    # Descarga el archivo en la carpeta temporal
    wget -qO "$TEMP_DIR/$GO_LATEST_VERSION" "$GO_DOWNLOAD_URL" #Descarga el archivo en el directorio temporal

    if [[ $? -ne 0 ]]; then
        echo "Error descargando Golang binaries. Exiting..."
        rm -rf "$TEMP_DIR" #Elimina el directorio temporal en caso de error
        return 1
    fi

    # Extrae el archivo desde la carpeta temporal
    tar -C /usr/local -xzf "$TEMP_DIR/$GO_LATEST_VERSION" #Extae el archivo desde el directorio temporal

    # Actualiza la variable PATH
    touch /etc/profile
    echo "export PATH=\$PATH:/usr/local/go/bin" >> /etc/profile

    # Si el usuario usa Zsh, también actualiza el archivo de configuración
    mkdir -p /etc/zsh
    touch /etc/zsh/zprofile
    echo "export PATH=\$PATH:/usr/local/go/bin" >> /etc/zsh/zprofile

    # Limpia la carpeta temporal
    rm -rf "$TEMP_DIR" #Borra la carpeta y todo su contenido.

    echo "Golang instalado correctamente."
    return 0
}

update_golang() {
    # Check if Go is installed
    if [[ ! -d "/usr/local/go" ]]; then
        echo "Go no está instalado. Ejecutando instalación limpia..."
        install_golang
        return
    fi

    # Create backup
    BACKUP_DIR="/tmp/go_backup_$(date +%Y%m%d_%H%M%S)"
    echo "Creando backup en: $BACKUP_DIR"
    if ! cp -r /usr/local/go "$BACKUP_DIR"; then
        echo "Error al crear backup. Abortando actualización..."
        return 1
    fi

    # Remove current installation
    echo "Removiendo instalación actual..."
    if ! rm -rf /usr/local/go; then
        echo "Error al remover instalación actual. Restaurando backup..."
        cp -r "$BACKUP_DIR" /usr/local/go
        rm -rf "$BACKUP_DIR"
        return 1
    fi

    # Install new version
    echo "Instalando nueva versión..."
    if ! install_golang; then
        echo "Error en la instalación. Restaurando backup..."
        cp -r "$BACKUP_DIR" /usr/local/go
        rm -rf "$BACKUP_DIR"
        return 1
    fi

    # Validate installation
    if ! command -v /usr/local/go/bin/go >/dev/null; then
        echo "Error: La nueva instalación no es válida. Restaurando backup..."
        rm -rf /usr/local/go
        cp -r "$BACKUP_DIR" /usr/local/go
        rm -rf "$BACKUP_DIR"
        return 1
    fi

    # Clean up backup on success
    echo "Actualización exitosa. Eliminando backup..."
    rm -rf "$BACKUP_DIR"
    echo "Go actualizado correctamente a: $(/usr/local/go/bin/go version)"
    return 0
}