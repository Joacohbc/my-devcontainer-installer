import type { DockerfileModule } from '@/types.js';

export const javaTemurinModule: DockerfileModule = {
  id: 'java-temurin',
  label: 'Java — Eclipse Temurin JDK + Maven',
  category: 'lang',
  conflicts: ['java-openjdk'],
  options: [
    {
      id: 'versions',
      label: 'JDK versions',
      type: 'multiselect',
      choices: [
        { value: '11', label: 'Temurin JDK 11' },
        { value: '17', label: 'Temurin JDK 17' },
        { value: '21', label: 'Temurin JDK 21' },
      ],
      default: ['11', '17'],
    },
    {
      id: 'maven',
      label: 'Install Maven',
      type: 'confirm',
      default: true,
    },
  ],
  render(opts) {
    const versions = (opts.versions as string[]) ?? ['11', '17'];
    const maven = opts.maven !== false;
    const pkgs = versions.map((v) => `temurin-${v}-jdk`).concat(maven ? ['maven'] : []);
    return `##
## JAVA (Temurin)
##
RUN wget -O - https://packages.adoptium.net/artifactory/api/gpg/key/public | apt-key add - && \\
    echo "deb https://packages.adoptium.net/artifactory/deb $(awk -F= '/^VERSION_CODENAME/{print$2}' /etc/os-release) main" | tee /etc/apt/sources.list.d/adoptium.list && \\
    apt-get update && \\
    apt-get install -y ${pkgs.join(' ')}
`;
  },
};
