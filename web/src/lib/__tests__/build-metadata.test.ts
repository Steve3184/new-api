import { beforeEach, expect, it, vi } from 'vitest'

beforeEach(() => {
  vi.resetModules()
})

it('prints the approved fork notice once when build metadata is installed repeatedly', async () => {
  const info = vi.spyOn(console, 'info').mockImplementation(() => {})
  vi.spyOn(console, 'debug').mockImplementation(() => {})
  const { installBuildMetadata } = await import('../build-metadata')

  installBuildMetadata()
  installBuildMetadata()

  expect(info).toHaveBeenCalledExactlyOnceWith(
    `[Third-Party Fork Notice]
This site runs Steve3184/new-api, a third-party fork of QuantumNous/new-api, not an official New API release.

To report bugs, suggest improvements, or contribute code, please visit:
https://github.com/Steve3184/new-api

Submit an issue:
https://github.com/Steve3184/new-api/issues/new

Please report issues here first. Do not submit bugs caused by this fork's additions or modifications to the upstream repository.`
  )
})

it('keeps startup working when the console info method throws', async () => {
  vi.spyOn(console, 'info').mockImplementation(() => {
    throw new Error('Console unavailable')
  })
  const debug = vi.spyOn(console, 'debug').mockImplementation(() => {})
  const { installBuildMetadata } = await import('../build-metadata')

  expect(() => installBuildMetadata()).not.toThrow()
  expect(document.documentElement.getAttribute('data-build-rev')).toBeTruthy()
  expect(debug).toHaveBeenCalledOnce()
})
