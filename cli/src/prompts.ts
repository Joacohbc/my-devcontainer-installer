import enquirer from 'enquirer';

export class PromptCancelledError extends Error {
  constructor() {
    super('Prompt cancelled');
  }
}

function isCancellation(e: unknown): boolean {
  if (e === undefined || e === null || e === '') return true;
  const err = e as { message?: string; code?: string };
  if (err.message === '' || err.message === 'canceled') return true;
  if (err.code === 'ERR_USE_AFTER_CLOSE') return true;
  return false;
}

async function safePrompt<T>(opts: object): Promise<T> {
  try {
    return await enquirer.prompt<T>(opts as never);
  } catch (e) {
    if (isCancellation(e)) throw new PromptCancelledError();
    throw e;
  }
}

export async function multiselect<T extends string = string>(
  name: string,
  message: string,
  choices: { name: T; message: string }[],
  initial: T[] = [],
): Promise<T[]> {
  const r = await safePrompt<Record<string, T[]>>({
    type: 'multiselect',
    name,
    message,
    choices: choices.map((c) => ({
      ...c,
      enabled: initial.includes(c.name),
    })),
  });
  return r[name];
}

export async function select<T extends string = string>(
  name: string,
  message: string,
  choices: { name: T; message: string }[],
  initial?: T,
): Promise<T> {
  const r = await safePrompt<Record<string, T>>({
    type: 'select',
    name,
    message,
    choices,
    initial: initial ? choices.findIndex((c) => c.name === initial) : 0,
  });
  return r[name];
}

export async function input(
  name: string,
  message: string,
  initial?: string,
  validate?: (v: string) => true | string,
): Promise<string> {
  const r = await safePrompt<Record<string, string>>({
    type: 'input',
    name,
    message,
    initial,
    validate,
  });
  return r[name];
}

export async function confirm(name: string, message: string, initial = true): Promise<boolean> {
  const r = await safePrompt<Record<string, boolean>>({
    type: 'confirm',
    name,
    message,
    initial,
  });
  return r[name];
}
