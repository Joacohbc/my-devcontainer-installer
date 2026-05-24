import chalk from 'chalk';

export const ui = {
  log(s: string) {
    console.log(`${chalk.blue.bold('==>')} ${s}`);
  },
  ok(s: string) {
    console.log(`${chalk.green.bold('✓')}   ${s}`);
  },
  warn(s: string) {
    console.log(`${chalk.yellow.bold('!')}   ${s}`);
  },
  success(s: string) {
    console.log(chalk.green.bold(s));
  },
  bar() {
    console.log(chalk.gray('─'.repeat(64)));
  },
};
