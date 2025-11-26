#!/bin/bash
# This script installs and configures Zsh with Oh My Zsh, popular plugins,
# and the Powerlevel10k theme. It also installs necessary fonts.
set -e

# Install dependencies
sudo apt-get update
sudo apt-get install -y git curl fontconfig

# Install Oh My Zsh non-interactively
sh -c "$(curl -fsSL https://raw.githubusercontent.com/ohmyzsh/ohmyzsh/master/tools/install.sh)" "" --unattended

# Clone Zsh plugins
git clone https://github.com/zsh-users/zsh-syntax-highlighting.git ${ZSH_CUSTOM:-~/.oh-my-zsh/custom}/plugins/zsh-syntax-highlighting
git clone https://github.com/zsh-users/zsh-history-substring-search ${ZSH_CUSTOM:-~/.oh-my-zsh/custom}/plugins/zsh-history-substring-search
git clone https://github.com/zsh-users/zsh-autosuggestions ${ZSH_CUSTOM:-~/.oh-my-zsh/custom}/plugins/zsh-autosuggestions

# Activate plugins in .zshrc
sed -i 's/plugins=(git)/plugins=(\n   git\n   zsh-history-substring-search\n   zsh-autosuggestions\n   zsh-syntax-highlighting\n)/' ~/.zshrc

# Download and install MesloLGS NF fonts
sudo curl -L -o /usr/local/share/fonts/MesloLGS_NF_Regular.ttf https://github.com/romkatv/powerlevel10k-media/raw/master/MesloLGS%20NF%20Regular.ttf
sudo curl -L -o /usr/local/share/fonts/MesloLGS_NF_Bold.ttf https://github.com/romkatv/powerlevel10k-media/raw/master/MesloLGS%20NF%20Bold.ttf
sudo curl -L -o /usr/local/share/fonts/MesloLGS_NF_Italic.ttf https://github.com/romkatv/powerlevel10k-media/raw/master/MesloLGS%20NF%20Italic.ttf
sudo curl -L -o /usr/local/share/fonts/MesloLGS_NF_Bold_Italic.ttf https://github.com/romkatv/powerlevel10k-media/raw/master/MesloLGS%20NF%20Bold%20Italic.ttf
sudo fc-cache -fv

# Install Powerlevel10k theme
git clone --depth=1 https://github.com/romkatv/powerlevel10k.git ${ZSH_CUSTOM:-$HOME/.oh-my-zsh/custom}/themes/powerlevel10k

# Activate Powerlevel10k theme in .zshrc
sed -i 's/ZSH_THEME="[^"]*"/ZSH_THEME="powerlevel10k\/powerlevel10k"/' ~/.zshrc

# Change default shell for devuser
sudo chsh -s $(which zsh) devuser
