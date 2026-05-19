import type { DockerfileModule } from '../../types.js';

type CliId =
  | 'claude-code'
  | 'opencode'
  | 'autoskills'
  | 'antigravity-cli'
  | 'copilot-cli';

const ALL: CliId[] = [
  'claude-code',
  'opencode',
  'autoskills',
  'antigravity-cli',
  'copilot-cli',
];

const SCRIPT_BY_TOOL: Record<CliId, string> = {
  'claude-code': 'install-claude-code.sh',
  opencode: 'install-opencode.sh',
  autoskills: 'install-autoskills.sh',
  'antigravity-cli': 'install-antigravity.sh',
  'copilot-cli': 'install-copilot.sh',
};

export const aiClisModule: DockerfileModule = {
  id: 'ai-clis',
  label: 'AI CLIs install scripts (Claude Code, OpenCode, Autoskills, Antigravity, Copilot CLI)',
  category: 'infra',
  requires: ['pnpm', 'github-cli'],
  copyFiles: [...Object.values(SCRIPT_BY_TOOL)],
  options: [
    {
      id: 'tools',
      label: 'AI CLIs to ship install scripts for',
      type: 'multiselect',
      choices: [
        { value: 'claude-code', label: 'Claude Code (native standalone installer)' },
        { value: 'opencode', label: 'OpenCode (opencode.ai installer)' },
        { value: 'autoskills', label: 'Autoskills (npx, no global install)' },
        { value: 'antigravity-cli', label: 'Antigravity CLI (native standalone installer)' },
        { value: 'copilot-cli', label: 'GitHub Copilot CLI (standalone binary)' },
      ],
      default: ALL,
    },
  ],
  render(opts) {
    const tools = normalizeTools(opts.tools);
    const installScripts = tools.map((t) => SCRIPT_BY_TOOL[t]);

    if (installScripts.length === 0) {
      return `##\n## AI CLIs — no tools selected\n##\n`;
    }

    const targets = installScripts.map((s) => `/home/devuser/${s}`).join(' ');
    return `##
## AI CLIs — per-tool install scripts
##
COPY ${installScripts.join(' ')} /home/devuser/
RUN chown devuser:devuser ${targets} && chmod +x ${targets}
`;
  },
};

function normalizeTools(raw: unknown): CliId[] {
  if (!Array.isArray(raw)) return ALL;
  const set = new Set(raw.map(String));
  return ALL.filter((t) => set.has(t));
}
