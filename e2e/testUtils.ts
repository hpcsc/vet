import { spawn, execFileSync } from 'node:child_process'
import { mkdtempSync, rmSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join, resolve } from 'node:path'
import { onTestFinished } from 'vitest'

export function getExecutablePath(): string {
  const executablePath = process.env.EXECUTABLE
  if (!executablePath) {
    throw new Error('EXECUTABLE environment variable is required. Set it to the path of the vet binary.')
  }
  return resolve(executablePath)
}

export function buildTag(): string {
  const tag = process.env.BUILD_TAG
  if (!tag) {
    throw new Error('BUILD_TAG environment variable is required. Set it to the tag the vet binary was built with.')
  }
  return tag
}

export function scratchDir(): string {
  const dir = mkdtempSync(join(tmpdir(), 'vet-e2e-'))
  onTestFinished(() => rmSync(dir, { recursive: true, force: true }))
  return dir
}

function git(cwd: string, args: string[]): void {
  execFileSync('git', args, { cwd, stdio: 'ignore' })
}

// gitRepoWithChange makes a repository with two commits and returns its
// directory: the first commit holds change.txt with one line, the second
// adds a line, so the base is the first commit.
export function gitRepoWithChange(): string {
  const dir = scratchDir()
  const change = join(dir, 'change.txt')
  git(dir, ['init', '-q', '-b', 'main'])
  git(dir, ['config', 'user.email', 'vet@e2e'])
  git(dir, ['config', 'user.name', 'vet'])
  git(dir, ['config', 'commit.gpgsign', 'false'])
  writeFileSync(change, 'one\n')
  git(dir, ['add', '.'])
  git(dir, ['commit', '-q', '-m', 'base'])
  writeFileSync(change, 'one\ntwo\n')
  git(dir, ['add', '.'])
  git(dir, ['commit', '-q', '-m', 'change'])
  return dir
}

export interface Result {
  stdout: string
  stderr: string
  status: number | null
}

// runCli does not block, so a fake server in this process can still answer the
// requests the CLI makes.
export function runCli(
  cwd: string,
  args: string[] = [],
  env: Record<string, string> = {},
  executable = getExecutablePath(),
): Promise<Result> {
  return new Promise((done, fail) => {
    const child = spawn(executable, args, { cwd, env: { ...process.env, ...env } })
    let stdout = ''
    let stderr = ''
    child.stdout.on('data', (chunk) => (stdout += chunk))
    child.stderr.on('data', (chunk) => (stderr += chunk))
    child.on('error', fail)
    child.on('close', (status) => done({ stdout, stderr, status }))
  })
}
