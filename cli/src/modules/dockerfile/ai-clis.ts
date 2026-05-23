import { POST_SCRIPT_DIR, type DockerfileModule } from '../../types.js';

type CliId =
  | 'claude-code'
  | 'opencode'
  | 'codex-cli'
  | 'antigravity-cli'
  | 'copilot-cli';

const ALL: CliId[] = [
  'claude-code',
  'opencode',
  'codex-cli',
  'antigravity-cli',
  'copilot-cli',
];

const SCRIPT_BY_TOOL: Record<CliId, string> = {
  'claude-code': 'install-claude-code.sh',
  opencode: 'install-opencode.sh',
  'codex-cli': 'install-codex-cli.sh',
  'antigravity-cli': 'install-antigravity.sh',
  'copilot-cli': 'install-copilot.sh',
};

export const aiClisModule: DockerfileModule = {
  id: 'ai-clis',
  label: 'AI CLIs install scripts (Claude Code, OpenCode, Codex CLI, Antigravity, Copilot CLI)',
  category: 'infra',
  requires: ['pnpm', 'github-cli'],
  // Shipped as post-install scripts: baked into POST_SCRIPT_DIR by the generator,
  // available inside the container, not auto-run.
  postScriptFiles: (opts) => normalizeTools(opts.tools).map((t) => SCRIPT_BY_TOOL[t]),
  options: [
    {
      id: 'tools',
      label: 'AI CLIs to ship install scripts for',
      type: 'multiselect',
      choices: [
        { value: 'claude-code', label: 'Claude Code (native standalone installer)' },
        { value: 'opencode', label: 'OpenCode (opencode.ai installer)' },
        { value: 'codex-cli', label: 'Codex CLI (npx, no global install)' },
        { value: 'antigravity-cli', label: 'Antigravity CLI (native standalone installer)' },
        { value: 'copilot-cli', label: 'GitHub Copilot CLI (standalone binary)' },
      ],
      default: ALL,
    },
  ],
  render() {
    return `##\n## AI CLIs — install scripts shipped under ${POST_SCRIPT_DIR}\n##\n`;
  },
};

function normalizeTools(raw: unknown): CliId[] {
  if (!Array.isArray(raw)) return ALL;
  const set = new Set(raw.map(String));
  return ALL.filter((t) => set.has(t));
}
