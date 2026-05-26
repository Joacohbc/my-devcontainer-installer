export const RC_FILES = ['.zshrc', '.bashrc', '.profile'];

const DEVUSER_HOME = '/home/devuser';

// Writes `lines` verbatim (no expansion of $ or ") to an init script in
// devuser's home and sources it from every interactive shell rc file. The
// default shell is zsh, which does not read .profile — so an installer that
// only appends to .profile leaves its tool unavailable in the default shell.
export function emitShellInit(initFileName: string, lines: string[]): string {
  for (const l of lines) {
    if (l.includes("'")) {
      throw new Error(`shell init line cannot contain single quotes: ${l}`);
    }
  }
  const initFile = `${DEVUSER_HOME}/${initFileName}`;
  const args = lines
    .map((l) => "'" + l.replace(/"/g, '\\"').replace(/\$/g, '\\$') + "'")
    .join(' ');
  const sourceLine = `. \\$HOME/${initFileName}`;
  return `RUN su - devuser -c "printf '%s\\n' ${args} > ${initFile}"
RUN su - devuser -c "for f in ${RC_FILES.join(' ')}; do touch ${DEVUSER_HOME}/\\$f && echo '${sourceLine}' >> ${DEVUSER_HOME}/\\$f; done"`;
}
