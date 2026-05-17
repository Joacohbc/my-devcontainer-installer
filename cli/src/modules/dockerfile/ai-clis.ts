import type { DockerfileModule } from '../../types.js';

type CliId = 'claude-code' | 'gemini' | 'opencode' | 'autoskills';

const ALL: CliId[] = ['claude-code', 'gemini', 'opencode', 'autoskills'];

const SCRIPT_BY_TOOL: Record<CliId, string> = {
  'claude-code': 'install-claude-code.sh',
  gemini: 'install-gemini.sh',
  opencode: 'install-opencode.sh',
  autoskills: 'install-autoskills.sh',
};

export const aiClisModule: DockerfileModule = {
  id: 'ai-clis',
  label: 'AI CLIs install scripts (Claude Code, Gemini, OpenCode, Autoskills)',
  category: 'infra',
  requires: ['pnpm', 'github-cli'],
  copyFiles: [...Object.values(SCRIPT_BY_TOOL)],
  options: [
    {
      id: 'tools',
      label: 'AI CLIs to ship install scripts for',
      type: 'multiselect',
      choices: [
        { value: 'claude-code', label: 'Claude Code (@anthropic-ai/claude-code)' },
        { value: 'gemini', label: 'Gemini CLI (@google/gemini-cli)' },
        { value: 'opencode', label: 'OpenCode (opencode.ai installer)' },
        { value: 'autoskills', label: 'Autoskills (npx, no global install)' },
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
