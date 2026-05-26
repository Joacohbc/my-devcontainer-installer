import { test } from 'node:test';
import assert from 'node:assert/strict';
import { isValidCidr, isValidImageName } from '@/domain/validators.js';

test('valid image names', () => {
  assert.ok(isValidImageName('foo'));
  assert.ok(isValidImageName('foo:latest'));
  assert.ok(isValidImageName('ghcr.io/joacohbc/devcontainer-ssh'));
  assert.ok(isValidImageName('devcontainer-ssh:local'));
});

test('invalid image names', () => {
  assert.ok(!isValidImageName('FOO'));
  assert.ok(!isValidImageName('foo bar'));
  assert.ok(!isValidImageName(''));
});

test('valid CIDRs', () => {
  assert.ok(isValidCidr('172.25.0.0/24'));
  assert.ok(isValidCidr('10.0.0.0/8'));
});

test('invalid CIDRs', () => {
  assert.ok(!isValidCidr('172.25.0.0'));
  assert.ok(!isValidCidr('999.0.0.0/24'));
  assert.ok(!isValidCidr('foo'));
});
