import assert from 'node:assert/strict'
import test from 'node:test'
import { requireHealthySplit } from './split-health.ts'

test('allows a healthy split video', () => {
  assert.doesNotThrow(() => requireHealthySplit({ status: 'ok' }))
})

test('blocks an unhealthy split video with repair guidance', () => {
  assert.throws(
    () => requireHealthySplit({ status: 'not_ok' }),
    /cannot be streamed because one or more split parts are missing or changed/i,
  )
})
