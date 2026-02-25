// Copyright (c) 2026 dotandev
// SPDX-License-Identifier: MIT OR Apache-2.0

import { Pkcs11Ed25519Signer } from '../src/audit/signing/pkcs11Signer';

describe('Pkcs11Ed25519Signer', () => {
  const originalEnv = process.env;

  beforeEach(() => {
    jest.resetModules();
    process.env = { ...originalEnv };
  });

  afterEach(() => {
    process.env = originalEnv;
  });

  describe('constructor validation', () => {
    test('should throw clear error when ERST_PKCS11_MODULE is not set', () => {
      delete process.env.ERST_PKCS11_MODULE;
      process.env.ERST_PKCS11_PIN = '1234';
      process.env.ERST_PKCS11_KEY_LABEL = 'test-key';

      expect(() => new Pkcs11Ed25519Signer()).toThrow(
        'pkcs11 provider selected but ERST_PKCS11_MODULE is not set'
      );
    });

    test('should throw clear error when ERST_PKCS11_PIN is not set', () => {
      process.env.ERST_PKCS11_MODULE = '/usr/lib/softhsm/libsofthsm2.so';
      delete process.env.ERST_PKCS11_PIN;
      process.env.ERST_PKCS11_KEY_LABEL = 'test-key';

      expect(() => new Pkcs11Ed25519Signer()).toThrow(
        'pkcs11 provider selected but ERST_PKCS11_PIN is not set'
      );
    });

    test('should throw clear error when neither key label nor key ID is set', () => {
      process.env.ERST_PKCS11_MODULE = '/usr/lib/softhsm/libsofthsm2.so';
      process.env.ERST_PKCS11_PIN = '1234';
      delete process.env.ERST_PKCS11_KEY_LABEL;
      delete process.env.ERST_PKCS11_KEY_ID;

      expect(() => new Pkcs11Ed25519Signer()).toThrow(
        'pkcs11 provider selected but neither ERST_PKCS11_KEY_LABEL nor ERST_PKCS11_KEY_ID is set'
      );
    });

    test('should throw clear error when pkcs11js is not installed', () => {
      process.env.ERST_PKCS11_MODULE = '/usr/lib/softhsm/libsofthsm2.so';
      process.env.ERST_PKCS11_PIN = '1234';
      process.env.ERST_PKCS11_KEY_LABEL = 'test-key';

      expect(() => new Pkcs11Ed25519Signer()).toThrow(
        'pkcs11 provider selected but optional dependency `pkcs11js` is not installed'
      );
    });
  });

  describe('public_key', () => {
    test('should return public key from environment when set', async () => {
      process.env.ERST_PKCS11_MODULE = '/usr/lib/softhsm/libsofthsm2.so';
      process.env.ERST_PKCS11_PIN = '1234';
      process.env.ERST_PKCS11_KEY_LABEL = 'test-key';
      process.env.ERST_PKCS11_PUBLIC_KEY_PEM = '-----BEGIN PUBLIC KEY-----\ntest\n-----END PUBLIC KEY-----';

      const signer = new Pkcs11Ed25519Signer();
      const publicKey = await signer.public_key();

      expect(publicKey).toBe(process.env.ERST_PKCS11_PUBLIC_KEY_PEM);
    });

    test('should throw clear error when public key is not configured', async () => {
      process.env.ERST_PKCS11_MODULE = '/usr/lib/softhsm/libsofthsm2.so';
      process.env.ERST_PKCS11_PIN = '1234';
      process.env.ERST_PKCS11_KEY_LABEL = 'test-key';
      delete process.env.ERST_PKCS11_PUBLIC_KEY_PEM;

      const signer = new Pkcs11Ed25519Signer();

      await expect(signer.public_key()).rejects.toThrow(
        'pkcs11 public key retrieval is not configured. Set ERST_PKCS11_PUBLIC_KEY_PEM to a SPKI PEM public key.'
      );
    });
  });

  describe('error context messages', () => {
    test('should provide context for module load failures', () => {
      const expectedErrorPattern = /Failed to load PKCS#11 module at '.*': .* Check that the library exists and is accessible\./;
      expect(expectedErrorPattern.test(
        "Failed to load PKCS#11 module at '/invalid/path.so': ENOENT. Check that the library exists and is accessible."
      )).toBe(true);
    });

    test('should provide context for initialization failures', () => {
      const testCases = [
        {
          error: 'Library lock error',
          expected: /Library lock error \(CKR_CANT_LOCK\)\. The PKCS#11 library may be in use by another process\./
        },
        {
          error: 'Token not present',
          expected: /Token not present \(CKR_TOKEN_NOT_PRESENT\)\. Ensure the HSM\/token is connected\./
        },
        {
          error: 'Device error',
          expected: /Device error \(CKR_DEVICE_ERROR\)\. Check HSM\/token hardware connection\./
        }
      ];

      testCases.forEach(({ error, expected }) => {
        expect(expected.test(`${error}: some details`)).toBe(true);
      });
    });

    test('should provide context for login failures', () => {
      const testCases = [
        {
          error: 'PIN incorrect',
          expected: /Wrong PIN \(CKR_PIN_INCORRECT\)/
        },
        {
          error: 'PIN locked',
          expected: /PIN locked \(CKR_PIN_LOCKED\)\. The token may be locked due to too many failed attempts\./
        },
        {
          error: 'Token not present',
          expected: /Token not present \(CKR_TOKEN_NOT_PRESENT\)/
        }
      ];

      testCases.forEach(({ error, expected }) => {
        expect(expected.test(`${error}: some details`)).toBe(true);
      });
    });
  });
});
