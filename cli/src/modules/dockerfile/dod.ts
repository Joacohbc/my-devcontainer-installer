import type { DockerfileModule } from '@/types.js';

export const dodModule: DockerfileModule = {
  id: 'dod',
  label: 'Docker-outside-Docker (DoD)',
  category: 'infra',
  render() {
    return `##
## DOCKER-OUTSIDE-DOCKER SETUP
##
RUN mkdir -p /etc/apt/keyrings
RUN curl -fsSL https://download.docker.com/linux/ubuntu/gpg | gpg --dearmor -o /etc/apt/keyrings/docker.gpg

RUN echo \\
    "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/ubuntu \\
    $(lsb_release -cs) stable" | tee /etc/apt/sources.list.d/docker.list > /dev/null

RUN apt-get update && apt-get install -y docker-ce-cli

RUN groupadd docker || true
RUN usermod -aG docker devuser
`;
  },
};
