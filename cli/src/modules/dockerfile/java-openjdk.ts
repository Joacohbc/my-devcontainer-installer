import type { DockerfileModule } from '../../types.js';

export const javaOpenjdkModule: DockerfileModule = {
  id: 'java-openjdk',
  label: 'Java — OpenJDK (Ubuntu repos) + Maven',
  category: 'lang',
  conflicts: ['java-temurin'],
  options: [
    {
      id: 'versions',
      label: 'JDK versions',
      type: 'multiselect',
      choices: [
        { value: '11', label: 'openjdk-11-jdk' },
        { value: '17', label: 'openjdk-17-jdk' },
        { value: '21', label: 'openjdk-21-jdk' },
      ],
      default: ['17'],
    },
    {
      id: 'maven',
      label: 'Install Maven',
      type: 'confirm',
      default: true,
    },
  ],
  render(opts) {
    const versions = (opts.versions as string[]) ?? ['17'];
    const maven = opts.maven !== false;
    const pkgs = versions.map((v) => `openjdk-${v}-jdk`).concat(maven ? ['maven'] : []);
    return `##
## JAVA (OpenJDK)
##
RUN apt-get update && apt-get install -y ${pkgs.join(' ')}
`;
  },
};
