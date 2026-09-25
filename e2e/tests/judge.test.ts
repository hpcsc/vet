import { writeFileSync } from 'node:fs'
import { createServer } from 'node:http'
import type { AddressInfo } from 'node:net'
import { join } from 'node:path'
import { describe, expect, it, onTestFinished } from 'vitest'
import { gitRepoWithChange, runCli, scratchDir } from '../testUtils'

const questionsFile = `version: 1
name: the rules
rules:
  - id: no-flag-field
    description: The change adds a flag field to the request struct.
    instructions: The change adds a flag field to the request struct.
    type: noul
    noulLimit: 0.5
  - id: database-migration
    description: How the change touches the database.
    instructions: Which option describes the change best?
    type: choice
    choices:
      no-db: The change does not touch the database.
      uses-db: The change reads or writes the database.
      migrates: The change alters the schema.
    violatesWhen: migrates
  - id: log-guideline
    description: How well the change follows the logging guideline.
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
  'database-migration': {
    type: 'choice',
    choice: 'uses-db',
    probabilities: { 'no-db': 0.01, 'uses-db': 0.94, migrates: 0.05 },
    confidence: 0.9,
  },
  'log-guideline': {
    type: 'score',
    score: 1,
    probabilities: { '0': 0.1, '1': 0.8, '2': 0.1 },
    legend: { '0': 'first', '1': 'second', '2': 'third' },
    confidence: 0.8,
  },
}

const violatingAnswers = {
  'no-flag-field': { type: 'noul', noul: 0.9 },
  'database-migration': {
    type: 'choice',
    choice: 'migrates',
    probabilities: { 'no-db': 0.0, 'uses-db': 0.1, migrates: 0.9 },
    confidence: 0.95,
  },
  'log-guideline': {
    type: 'score',
    score: 3,
    probabilities: { '0': 0.0, '1': 0.0, '2': 0.2, '3': 0.8 },
    legend: { '0': 'first', '1': 'second', '2': 'third', '3': 'fourth' },
    confidence: 0.9,
  },
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
  it('hides passing rules when a change violates no rule and exits 0', async () => {
    const repo = gitRepoWithChange()
    writeFileSync(join(repo, 'questions.yaml'), questionsFile)
    const api = await fakeSystemOne(cleanAnswers)

    const result = await runCli(repo, judgeArgs(api))

    expect(result.status).toBe(0)
    expect(result.stdout).not.toContain('✓ [noul]')
    expect(result.stdout).toContain('The change violates no rule.')
  })

  it('shows passing rules with --all', async () => {
    const repo = gitRepoWithChange()
    writeFileSync(join(repo, 'questions.yaml'), questionsFile)
    const api = await fakeSystemOne(cleanAnswers)

    const result = await runCli(repo, [...judgeArgs(api), '--all'])

    expect(result.status).toBe(0)
    expect(result.stdout).toContain('the rules')
    expect(result.stdout).toContain('✓ [noul] The change adds a flag field to the request struct.: 20%')
    expect(result.stdout).toContain('The change violates no rule.')
  })

  it('reports a violating change and exits 0 without --exit-code', async () => {
    const repo = gitRepoWithChange()
    writeFileSync(join(repo, 'questions.yaml'), questionsFile)
    const api = await fakeSystemOne(violatingAnswers)

    const result = await runCli(repo, judgeArgs(api))

    expect(result.status).toBe(0)
    expect(result.stdout).toContain('✗ [choice] How the change touches the database.: migrates (The change alters the schema.) (confidence 0.95)')
    expect(result.stdout).toContain('The change violates 3 rules.')
    expect(result.stdout).toContain('- How the change touches the database. in change.txt (the rules)')
  })

  it('exits 1 when a change violates a rule and --exit-code is on', async () => {
    const repo = gitRepoWithChange()
    writeFileSync(join(repo, 'questions.yaml'), questionsFile)
    const api = await fakeSystemOne(violatingAnswers)

    const result = await runCli(repo, [...judgeArgs(api), '--exit-code'])

    expect(result.status).toBe(1)
    expect(result.stderr).toBe('')
  })

  it('hides passing answers in JSON by default', async () => {
    const repo = gitRepoWithChange()
    writeFileSync(join(repo, 'questions.yaml'), questionsFile)
    const api = await fakeSystemOne(cleanAnswers)

    const result = await runCli(repo, [...judgeArgs(api), '--json'])

    expect(result.status).toBe(0)
    const report = JSON.parse(result.stdout)
    expect(report.base).toBe('HEAD~1')
    expect(report.violations).toBe(0)
    expect(report.groups).toEqual([])
  })

  it('prints all answers in JSON with --all', async () => {
    const repo = gitRepoWithChange()
    writeFileSync(join(repo, 'questions.yaml'), questionsFile)
    const api = await fakeSystemOne(cleanAnswers)

    const result = await runCli(repo, [...judgeArgs(api), '--json', '--all'])

    expect(result.status).toBe(0)
    const report = JSON.parse(result.stdout)
    expect(report.base).toBe('HEAD~1')
    expect(report.violations).toBe(0)
    expect(report.groups).toEqual([
      {
        name: 'the rules',
        answers: [
          {
            path: 'change.txt',
            rule: 'no-flag-field',
            description: 'The change adds a flag field to the request struct.',
            value: 0.2,
          },
          {
            path: 'change.txt',
            rule: 'database-migration',
            description: 'How the change touches the database.',
            value: 'uses-db',
            label: 'The change reads or writes the database.',
            confidence: 0.9,
            probabilities: { 'no-db': 0.01, 'uses-db': 0.94, migrates: 0.05 },
          },
          {
            path: 'change.txt',
            rule: 'log-guideline',
            description: 'How well the change follows the logging guideline.',
            value: 1,
            label: 'second',
            confidence: 0.8,
            probabilities: { '0': 0.1, '1': 0.8, '2': 0.1 },
            legend: { '0': 'first', '1': 'second', '2': 'third' },
          },
        ],
      },
    ])
  })

  it('does not ask the backend about a file that no rule applies to', async () => {
    const repo = gitRepoWithChange()
    const goOnly = `version: 1
name: go rules
rules:
  - id: go-naming
    description: The change names the Go identifiers well.
    instructions: Rate how well the change names the identifiers.
    type: score
    scores:
      - well
      - poorly
    scoreLimit: 1
    files:
      - '**/*.go'
`
    writeFileSync(join(repo, 'questions.yaml'), goOnly)
    let hits = 0
    const server = createServer((_request, response) => {
      hits++
      response.statusCode = 500
      response.end('a request should not have been made')
    })
    await new Promise<void>((done) => server.listen(0, '127.0.0.1', done))
    onTestFinished(() => new Promise<void>((done) => server.close(() => done())))
    const api = `http://127.0.0.1:${(server.address() as AddressInfo).port}`

    const result = await runCli(repo, judgeArgs(api))

    expect(result.status).toBe(0)
    expect(result.stdout).toContain('The change violates no rule.')
    expect(hits).toBe(0)
  })

  it('fails with exit 2 when no API key is set', async () => {
    const repo = gitRepoWithChange()
    writeFileSync(join(repo, 'questions.yaml'), questionsFile)
    const api = await fakeSystemOne(cleanAnswers)
    const configHome = scratchDir()

    const result = await runCli(
      repo,
      ['--base', 'HEAD~1', '--api-url', api, '--questions', 'questions.yaml'],
      { XDG_CONFIG_HOME: configHome, TYPESAFE_API_KEY: '' },
    )

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