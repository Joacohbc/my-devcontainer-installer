##
## NVM SETUP (Node Version Manager)
##

# Install NVM for devuser (last step to avoid issues with other installations)
RUN NVM_VERSION=$(curl -s https://api.github.com/repos/nvm-sh/nvm/releases/latest | jq -r .tag_name) && \
    su - devuser -c "curl -o- https://raw.githubusercontent.com/nvm-sh/nvm/${NVM_VERSION}/install.sh | bash"
RUN su - devuser -c 'echo "export NVM_DIR=\"\$([ -z \"\${XDG_CONFIG_HOME-}\" ] && printf %s \"\${HOME}/.nvm\" || printf %s \"\${XDG_CONFIG_HOME}/nvm\")\"\n[ -s \"\$NVM_DIR/nvm.sh\" ] && \. \"\$NVM_DIR/nvm.sh\"" >> /home/devuser/.profile'
RUN su - devuser -c 'export NVM_DIR="$HOME/.nvm" && source "$NVM_DIR/nvm.sh" && nvm install --lts'
RUN su - devuser -c 'echo "nvm use --lts >> /dev/null" >> /home/devuser/.profile'

# Install PNPM for devuser
RUN su - devuser -c 'wget -qO- https://get.pnpm.io/install.sh | ENV="$HOME/.profile" SHELL="$(which zsh)" zsh -'
