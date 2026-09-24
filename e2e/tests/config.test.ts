import { chmodSync, readFileSync, writeFileSync } from 'node:fs'
import { createServer } from 'node:http'
import type { AddressInfo } from 'node:net'
import { join } from 'node:path'
import { describe, expect, it, onTestFinished } from 'vitest'
import { gitRepoWithChange, runCli, scratchDir } from '../testUtils'

const questionsFile = `version: 1
rules:
  - id: no-flag-field
    instructions: The change adds a flag field to the request struct.
    type: noul
    noulLimit: 0.5
`

async function fakeSystemOne(key: string, model = 'jev-latest'): Promise<string> {
  const server = createServer((request, response) => {
    let body = ''
    request.on('data', (chunk) => (body += chunk))
    request.on('end', () => {
      const req = JSON.parse(body)
      expect(request.headers['authorization']).toBe(`Bearer ${key}`)
      expect(req.model).toBe(model)
      response.setHeader('content-type', 'application/json')
      response.end(
        JSON.stringify({
          answers: { 'no-flag-field': { type: 'noul', noul: 0.2 } },
        }),
      )
    })
  })
  await new Promise<void>((done) => server.listen(0, '127.0.0.1', done))
  onTestFinished(() => new Promise<void>((done) => server.close(() => done())))
  return `http://127.0.0.1:${(server.address() as AddressInfo).port}`
}

describe('vet config', () => {
  it('prints a valid default config file', async () => {
    const result = await runCli(scratchDir(), ['config'])

    expect(result.status).toBe(0)
    expect(result.stdout).toContain('apiKeyCommand:')
    expect(result.stdout).toContain('questionsFile:')
    expect(result.stdout).toContain('apiUrl:')
    expect(result.stdout).toContain('model:')
  })
})

describe('vet questions init', () => {
  it('writes a default questions file with a rule of each type', async () => {
    const dir = scratchDir()
    const path = join(dir, 'questions.yaml')

    const result = await runCli(dir, ['questions', 'init', '--path', path])

    expect(result.status).toBe(0)
    const file = readFileSync(path, 'utf8')
    expect(file).toContain('type: noul')
    expect(file).toContain('type: choice')
    expect(file).toContain('type: score')
  })

  it('refuses to overwrite an existing file without --force', async () => {
    const dir = scratchDir()
    const path = join(dir, 'questions.yaml')
    writeFileSync(path, 'version: 1\nrules: []\n')

    const result = await runCli(dir, ['questions', 'init', '--path', path])

    expect(result.status).toBe(2)
    expect(readFileSync(path, 'utf8')).toBe('version: 1\nrules: []\n')
  })
})

describe('the config file', () => {
  it('provides the API key through an api-key-command', async () => {
    const repo = gitRepoWithChange()
    writeFileSync(join(repo, 'questions.yaml'), questionsFile)
    const keyDir = scratchDir()
    const keyScript = join(keyDir, 'key.sh')
    writeFileSync(keyScript, '#!/bin/sh\nprintf "from-script"\n')
    chmodSync(keyScript, 0o755)
    const api = await fakeSystemOne('from-script')
    const config = join(repo, 'config.yaml')
    writeFileSync(config, `apiKeyCommand: ${keyScript}\napiUrl: ${api}\n`)

    const result = await runCli(repo, ['--config', config, '--base', 'HEAD~1', '--questions', 'questions.yaml'])

    expect(result.status).toBe(0)
  })

  it('reaches the judge through the api-url and model and questionsFile', async () => {
    const repo = gitRepoWithChange()
    writeFileSync(join(repo, 'rules.yaml'), questionsFile)
    const api = await fakeSystemOne('env-key', 'rowan')
    const config = join(repo, 'config.yaml')
    writeFileSync(
      config,
      `apiUrl: ${api}
model: rowan
questionsFile: rules.yaml
`,
    )

    const result = await runCli(repo, ['--config', config, '--base', 'HEAD~1'], { TYPESAFE_API_KEY: 'env-key' })

    expect(result.status).toBe(0)
  })
})