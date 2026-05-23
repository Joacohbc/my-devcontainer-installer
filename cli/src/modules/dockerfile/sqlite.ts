import type { DockerfileModule } from '@/types.js';

export const sqliteModule: DockerfileModule = {
  id: 'sqlite',
  label: 'SQLite',
  category: 'db',
  render() {
    return `##
## SQLITE
##
RUN apt-get update && apt-get install -y sqlite3
`;
  },
};
