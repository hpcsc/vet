import { writeFileSync } from 'node:fs'
import { createServer } from 'node:http'
import type { AddressInfo } from 'node:net'
import { join } from 'node:path'
import { describe, expect, it, onTestFinished } from 'vitest'
import { gitRepoWithChange, runCli } from '../testUtils'

const questionsFile = `version: 1
name: the rules
rules:
  - id: no-flag-field
    instructions: The change adds a flag field to the request struct.
    type: noul
    noulLimit: 0.5
  - id: database-migration
    instructions: Which option describes the change best?
    type: choice
    choices:
      no-db: The change does not touch the database.
      uses-db: The change reads or writes the database.
      migrates: The change alters the schema.
    violatesWhen: migrates
  - id: log-guideline
    instructions: Rate how the change follows the logging guideline.
    type: score
    scores:
      - first
      - second
      - third
    scoreLimit: 2
`

const cleanAnswers = {
  'no-flag-field': { type: 'noul', noul: 0.2 },
  'database-migration': { type: 'choice', choice: 'uses-db', confidence: 0.9 },
  'log-guideline': { type: 'score', score: 1, confidence: 0.8 },
}

const violatingAnswers = {
  'no-flag-field': { type: 'noul', noul: 0.9 },
  'database-migration': { type: 'choice', choice: 'migrates', confidence: 0.9 },
  'log-guideline': { type: 'score', score: 3, confidence: 0.8 },
}

// fakeSystemOne answers judge requests. It checks the request asks exactly the
// rules the test expects, and answers each one.
async function fakeSystemOne(answers: Record<string, Record<string, unknown>>): Promise<string> {
  const expected = Object.keys(answers).sort()
  const server = createServer((request, response) => {
    let body = ''
    request.on('data', (chunk) => (body += chunk))
    request.on('end', () => {
      try {
        const req = JSON.parse(body)
        expect(req.model).toBe('jev-latest')
        expect(req.state).toContain('File: change.txt')
        expect(Object.keys(req.questions).sort()).toEqual(expected)
        response.setHeader('content-type', 'application/json')
        response.end(JSON.stringify({ answers }))
      } catch (error) {
        response.statusCode = 500
        response.end(String(error))
      }
    })
  })
  await new Promise<void>((done) => server.listen(0, '127.0.0.1', done))
  onTestFinished(() => new Promise<void>((done) => server.close(() => done())))
  return `http://127.0.0.1:${(server.address() as AddressInfo).port}`
}

function judgeArgs(api: string): string[] {
  return ['--base', 'HEAD~1', '--api-key', 'test', '--api-url', api, '--questions', 'questions.yaml']
}

describe('the judge', () => {
  it('reports a change that violates no rule and exits 0', async () => {
    const repo = gitRepoWithChange()
    writeFileSync(join(repo, 'questions.yaml'), questionsFile)
    const api = await fakeSystemOne(cleanAnswers)

    const result = await runCli(repo, judgeArgs(api))

    expect(result.status).toBe(0)
    expect(result.stdout).toContain('the rules')
    expect(result.stdout).toContain('✓ no-flag-field: 0.2')
    expect(result.stdout).toContain('The change violates no rule.')
  })

  it('reports a violating change and exits 0 without --exit-code', async () => {
    const repo = gitRepoWithChange()
    writeFileSync(join(repo, 'questions.yaml'), questionsFile)
    const api = await fakeSystemOne(violatingAnswers)

    const result = await runCli(repo, judgeArgs(api))

    expect(result.status).toBe(0)
    expect(result.stdout).toContain('✗ database-migration: migrates')
    expect(result.stdout).toContain('The change violates 3 rules.')
    expect(result.stdout).toContain('- database-migration in change.txt (the rules)')
  })

  it('exits 1 when a change violates a rule and --exit-code is on', async () => {
    const repo = gitRepoWithChange()
    writeFileSync(join(repo, 'questions.yaml'), questionsFile)
    const api = await fakeSystemOne(violatingAnswers)

    const result = await runCli(repo, [...judgeArgs(api), '--exit-code'])

    expect(result.status).toBe(1)
    expect(result.stderr).toBe('')
  })

  it('prints the report as JSON with --json', async () => {
    const repo = gitRepoWithChange()
    writeFileSync(join(repo, 'questions.yaml'), questionsFile)
    const api = await fakeSystemOne(cleanAnswers)

    const result = await runCli(repo, [...judgeArgs(api), '--json'])

    expect(result.status).toBe(0)
    const report = JSON.parse(result.stdout)
    expect(report.base).toBe('HEAD~1')
    expect(report.violations).toBe(0)
    expect(report.groups).toEqual([
      {
        name: 'the rules',
        answers: [
          { path: 'change.txt', rule: 'no-flag-field', value: 0.2 },
          { path: 'change.txt', rule: 'database-migration', value: 'uses-db', confidence: 0.9 },
          { path: 'change.txt', rule: 'log-guideline', value: 1, confidence: 0.8 },
        ],
      },
    ])
  })

  it('fails with exit 2 when no API key is set', async () => {
    const repo = gitRepoWithChange()
    writeFileSync(join(repo, 'questions.yaml'), questionsFile)
    const api = await fakeSystemOne(cleanAnswers)

    const result = await runCli(repo, ['--base', 'HEAD~1', '--api-url', api, '--questions', 'questions.yaml'])

    expect(result.status).toBe(2)
    expect(result.stdout).toContain('no API key')
  })

  it('fails with exit 2 when the questions file is missing', async () => {
    const repo = gitRepoWithChange()
    const api = await fakeSystemOne(cleanAnswers)

    const result = await runCli(repo, ['--base', 'HEAD~1', '--api-key', 'test', '--api-url', api, '--questions', 'no.yaml'])

    expect(result.status).toBe(2)
  })

  it('fails with exit 2 when the backend errors', async () => {
    const repo = gitRepoWithChange()
    writeFileSync(join(repo, 'questions.yaml'), questionsFile)
    const server = createServer((_request, response) => {
      response.statusCode = 500
      response.end('boom')
    })
    await new Promise<void>((done) => server.listen(0, '127.0.0.1', done))
    onTestFinished(() => new Promise<void>((done) => server.close(() => done())))
    const api = `http://127.0.0.1:${(server.address() as AddressInfo).port}`

    const result = await runCli(repo, judgeArgs(api))

    expect(result.status).toBe(2)
    expect(result.stdout).toContain('the backend answered')
  })
})