#!/bin/bash

# Actualizar el sistema e instalar las dependencias necesarias
sudo apt-get update
sudo apt-get install git zsh curl fontconfig -y

echo "> Luego de instalarse Oh My Zsh, debe salir (exit) para continuar..."
# Instalar Oh My Zsh
sh -c "$(curl -fsSL https://raw.githubusercontent.com/ohmyzsh/ohmyzsh/master/tools/install.sh)"

# Clonar los plugins de Zsh en el directorio de plugins de Oh My Zsh
git clone https://github.com/zsh-users/zsh-syntax-highlighting.git ${ZSH_CUSTOM:-~/.oh-my-zsh/custom}/plugins/zsh-syntax-highlighting
git clone https://github.com/zsh-users/zsh-history-substring-search ${ZSH_CUSTOM:-~/.oh-my-zsh/custom}/plugins/zsh-history-substring-search
git clone https://github.com/zsh-users/zsh-autosuggestions ${ZSH_CUSTOM:-~/.oh-my-zsh/custom}/plugins/zsh-autosuggestions

# Editar el archivo .zshrc para activar los plugins
sed -i 's/plugins=(git)/plugins=(\n   git\n   zsh-history-substring-search\n   zsh-autosuggestions\n   zsh-syntax-highlighting\n)/' ~/.zshrc

# Descargar las fuentes MesloLGS NF en el directorio de fuentes del sistema
curl -L -o /usr/local/share/fonts/MesloLGS_NF_Regular.ttf https://github.com/romkatv/powerlevel10k-media/raw/master/MesloLGS%20NF%20Regular.ttf 
curl -L -o /usr/local/share/fonts/MesloLGS_NF_Bold.ttf https://github.com/romkatv/powerlevel10k-media/raw/master/MesloLGS%20NF%20Bold.ttf
curl -L -o /usr/local/share/fonts/MesloLGS_NF_Italic.ttf https://github.com/romkatv/powerlevel10k-media/raw/master/MesloLGS%20NF%20Italic.ttf
curl -L -o /usr/local/share/fonts/MesloLGS_NF_Bold_Italic.ttf https://github.com/romkatv/powerlevel10k-media/raw/master/MesloLGS%20NF%20Bold%20Italic.ttf

# Actualizar la caché de fuentes del sistema
fc-cache -fv

# Clonar el tema Powerlevel10k en el directorio de temas de Oh My Zsh
git clone --depth=1 https://github.com/romkatv/powerlevel10k.git ${ZSH_CUSTOM:-$HOME/.oh-my-zsh/custom}/themes/powerlevel10k

# Editar el archivo .zshrc para activar el tema Powerlevel10k
sed -i 's/ZSH_THEME="[^"]*"/ZSH_THEME="powerlevel10k\/powerlevel10k"/' ~/.zshrc

echo '> Ahora solo ejecuta cierra sesion en esta shell y entra nuevamente para configurar p10k (o ejecuta p10k configure)...'