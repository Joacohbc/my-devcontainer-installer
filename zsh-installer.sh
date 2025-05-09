#!/bin/bash
# This script installs and configures Zsh with Oh My Zsh, popular plugins,
# and the Powerlevel10k theme. It also installs necessary fonts.

# Update the system and install necessary dependencies
apt-get update
apt-get install git zsh curl fontconfig -y

echo "> After Oh My Zsh is installed, you must exit to continue..."
# Install Oh My Zsh
# The -y --unattended flags are not standard for this script, if it causes issues, remove them.
sh -c "$(curl -fsSL https://raw.githubusercontent.com/ohmyzsh/ohmyzsh/master/tools/install.sh)" "" --unattended

# Clone Zsh plugins into the Oh My Zsh custom plugins directory
git clone https://github.com/zsh-users/zsh-syntax-highlighting.git ${ZSH_CUSTOM:-~/.oh-my-zsh/custom}/plugins/zsh-syntax-highlighting
git clone https://github.com/zsh-users/zsh-history-substring-search ${ZSH_CUSTOM:-~/.oh-my-zsh/custom}/plugins/zsh-history-substring-search
git clone https://github.com/zsh-users/zsh-autosuggestions ${ZSH_CUSTOM:-~/.oh-my-zsh/custom}/plugins/zsh-autosuggestions

# Edit the .zshrc file to activate the plugins
sed -i 's/plugins=(git)/plugins=(\n   git\n   zsh-history-substring-search\n   zsh-autosuggestions\n   zsh-syntax-highlighting\n)/' ~/.zshrc

# Download MesloLGS NF fonts to the system fonts directory
curl -L -o /usr/local/share/fonts/MesloLGS_NF_Regular.ttf https://github.com/romkatv/powerlevel10k-media/raw/master/MesloLGS%20NF%20Regular.ttf
curl -L -o /usr/local/share/fonts/MesloLGS_NF_Bold.ttf https://github.com/romkatv/powerlevel10k-media/raw/master/MesloLGS%20NF%20Bold.ttf
curl -L -o /usr/local/share/fonts/MesloLGS_NF_Italic.ttf https://github.com/romkatv/powerlevel10k-media/raw/master/MesloLGS%20NF%20Italic.ttf
curl -L -o /usr/local/share/fonts/MesloLGS_NF_Bold_Italic.ttf https://github.com/romkatv/powerlevel10k-media/raw/master/MesloLGS%20NF%20Bold%20Italic.ttf

# Update the system font cache
fc-cache -fv

# Clone the Powerlevel10k theme into the Oh My Zsh custom themes directory
git clone --depth=1 https://github.com/romkatv/powerlevel10k.git ${ZSH_CUSTOM:-$HOME/.oh-my-zsh/custom}/themes/powerlevel10k

# Edit the .zshrc file to activate the Powerlevel10k theme
sed -i 's/ZSH_THEME="[^"]*"/ZSH_THEME="powerlevel10k\/powerlevel10k"/' ~/.zshrc

echo '> Now, just log out of this shell and log back in to configure p10k (or run p10k configure)...'
chsh -s $(which zsh)