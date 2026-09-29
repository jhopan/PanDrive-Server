import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import test from 'node:test'

const source = await readFile(new URL('./SettingsPage.tsx', import.meta.url), 'utf8')

test('SettingsPage exposes encrypted off-VPS backup controls', () => {
  assert.match(source, /\/settings\/backup-drive/)
  assert.match(source, /\/settings\/backup-drive\/run/)
  assert.match(source, /Encrypted off-VPS backup/)
  assert.match(source, /Run backup now/)
  assert.match(source, /keeps seven latest backups/i)
})
