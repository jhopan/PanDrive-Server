import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import test from 'node:test'

const source = await readFile(new URL('./SettingsPage.tsx', import.meta.url), 'utf8')

test('SettingsPage shows dependency audit metadata and guidance without a vulnerability claim', () => {
  assert.match(source, /\/system\/dependencies/)
  assert.match(source, /Dependency audit/)
  assert.match(source, /No vulnerability scan has run on this server/)
  assert.match(source, /npm audit/)
  assert.match(source, /govulncheck/)
})
