import { describe, expect, it } from 'vitest'
import { buildTag, runCli, scratchDir } from '../testUtils'

describe('vet questions example', () => {
  it('prints a questions file with a rule of each type', async () => {
    const result = await runCli(scratchDir(), ['questions', 'example'])

    expect(result.status).toBe(0)
    expect(result.stdout).toContain('type: noul')
    expect(result.stdout).toContain('type: choice')
    expect(result.stdout).toContain('type: score')
  })
})

describe('vet version', () => {
  it('prints the tag the binary was built with', async () => {
    const result = await runCli(scratchDir(), ['version'])

    expect(result.status).toBe(0)
    expect(result.stdout.trim()).toBe(buildTag())
  })

  it('--version prints the name and the tag', async () => {
    const result = await runCli(scratchDir(), ['--version'])

    expect(result.status).toBe(0)
    expect(result.stdout.trim()).toBe(`vet version ${buildTag()}`)
  })
})