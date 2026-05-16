import type { DockerfileModule } from '../../types.js';

export const baseModule: DockerfileModule = {
  id: 'base',
  label: 'Base (Ubuntu + SSH + zsh + sudo)',
  category: 'base',
  always: true,
  copyFiles: ['zsh-installer.sh'],
  options: [
    {
      id: 'ubuntu',
      label: 'Ubuntu version',
      type: 'select',
      choices: [
        { value: '24.04', label: '24.04 (Noble)' },
        { value: '22.04', label: '22.04 (Jammy)' },
      ],
      default: '24.04',
    },
  ],
  render(opts) {
    const ubuntu = (opts.ubuntu as string) || '24.04';
    return `# Use an Ubuntu base image
FROM ubuntu:${ubuntu}

# Update packages and install SSH, sudo, and other utilities
RUN apt-get update && export DEBIAN_FRONTEND=noninteractive \\
    && apt-get -y install --no-install-recommends \\
    openssh-server \\
    nano \\
    sudo \\
    pwgen \\
    zsh \\
    fontconfig \\
    ca-certificates \\
    curl \\
    gnupg \\
    lsb-release \\
    acl \\
    git \\
    wget \\
    unzip \\
    apt-transport-https

# Create devuser with sudo privileges
RUN useradd -m -s /bin/zsh devuser && \\
    usermod -aG sudo devuser && \\
    echo "devuser ALL=(ALL) NOPASSWD:ALL" >> /etc/sudoers

# Install Zsh configuration and plugins
COPY zsh-installer.sh /tmp/zsh-installer.sh
RUN chmod +x /tmp/zsh-installer.sh
RUN su - devuser -c "/tmp/zsh-installer.sh"
RUN rm /tmp/zsh-installer.sh
`;
  },
};
