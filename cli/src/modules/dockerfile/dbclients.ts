import type { DockerfileModule } from '../../types.js';

export const dbclientsModule: DockerfileModule = {
  id: 'dbclients',
  label: 'Database clients (psql, redis-cli, mongosh)',
  category: 'db',
  options: [
    {
      id: 'clients',
      label: 'Which clients',
      type: 'multiselect',
      choices: [
        { value: 'postgres', label: 'postgresql-client' },
        { value: 'redis', label: 'redis-tools' },
        { value: 'mongo', label: 'mongodb-mongosh' },
      ],
      default: ['postgres', 'redis', 'mongo'],
    },
  ],
  render(opts) {
    const clients = (opts.clients as string[]) ?? ['postgres', 'redis', 'mongo'];
    const pkgs: string[] = [];
    if (clients.includes('postgres')) pkgs.push('postgresql-client');
    if (clients.includes('redis')) pkgs.push('redis-tools');

    const mongoSetup = clients.includes('mongo')
      ? `RUN curl -fsSL https://www.mongodb.org/static/pgp/server-8.0.asc | gpg --dearmor -o /etc/apt/keyrings/mongodb-server-8.0.gpg && \\
    echo "deb [ arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/mongodb-server-8.0.gpg ] https://repo.mongodb.org/apt/ubuntu jammy/mongodb-org/8.0 multiverse" | tee /etc/apt/sources.list.d/mongodb-org-8.0.list
`
      : '';
    if (clients.includes('mongo')) pkgs.push('mongodb-mongosh');

    if (pkgs.length === 0) return '';

    return `##
## DATABASE CLIENTS
##
${mongoSetup}RUN apt-get update && apt-get install -y \\
    ${pkgs.join(' \\\n    ')}
`;
  },
};
