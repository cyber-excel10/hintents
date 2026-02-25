// Copyright (c) Hintents Authors.
// SPDX-License-Identifier: Apache-2.0

import type { AuditSigner, PublicKey, Signature } from './types';
import { HsmRateLimiter } from './rateLimiter';

// eslint-disable-next-line @typescript-eslint/no-var-requires
const lazyRequire = (name: string): any => {
  // Using eval('require') keeps this file compatible with CommonJS builds and avoids TS/webpack rewriting.
  // eslint-disable-next-line no-eval
  return eval('require')(name);
};

const TOKEN_LABEL_PADDING = /\0/g;
const PIV_SLOT_REGEX = /^(0x)?[0-9a-fA-F]{2}$/;

export const normalizeTokenLabel = (label: string): string =>
  label.replace(TOKEN_LABEL_PADDING, '').trim();

export const resolveYkcs11KeyIdHex = (pivSlot: string): string => {
  const trimmed = pivSlot.trim().toLowerCase();
  if (!PIV_SLOT_REGEX.test(trimmed)) {
    throw new Error(`Invalid PIV slot '${pivSlot}'. Expected a 2-digit hex value like 9a, 9c, or f9.`);
  }

  const hex = trimmed.startsWith('0x') ? trimmed.slice(2) : trimmed;
  const slotValue = Number.parseInt(hex, 16);

  let keyId: number | undefined;
  if (slotValue === 0x9a) keyId = 1;
  if (slotValue === 0x9c) keyId = 2;
  if (slotValue === 0x9d) keyId = 3;
  if (slotValue === 0x9e) keyId = 4;
  if (slotValue >= 0x82 && slotValue <= 0x95) keyId = slotValue - 0x82 + 5;
  if (slotValue === 0xf9) keyId = 25;

  if (!keyId) {
    throw new Error(
      `Unsupported PIV slot '${pivSlot}'. Supported slots: 9a, 9c, 9d, 9e, 82-95, f9.`
    );
  }

  return keyId.toString(16).padStart(2, '0');
};

export const resolvePkcs11KeyIdHex = (cfg: { keyIdHex?: string; pivSlot?: string }): string | undefined => {
  if (cfg.keyIdHex) {
    const normalized = cfg.keyIdHex.trim();
    if (!/^[0-9a-fA-F]+$/.test(normalized) || normalized.length % 2 !== 0) {
      throw new Error(
        `Invalid ERST_PKCS11_KEY_ID '${cfg.keyIdHex}'. Expected an even-length hex string (e.g., 01, 0a, 10).`
      );
    }
    return normalized;
  }
  if (cfg.pivSlot) return resolveYkcs11KeyIdHex(cfg.pivSlot);
  return undefined;
};

const resolvePkcs11Slot = (opts: {
  slots: number[];
  slotIndex?: string;
  tokenLabel?: string;
  getTokenInfo: (slotId: number) => { label?: string };
}): number | undefined => {
  if (opts.tokenLabel) {
    const desired = normalizeTokenLabel(opts.tokenLabel);
    const available: string[] = [];
    for (const slot of opts.slots) {
      const info = opts.getTokenInfo(slot);
      const label = info?.label ? normalizeTokenLabel(info.label) : '';
      if (label) available.push(label);
      if (label && label === desired) return slot;
    }

    if (available.length > 0) {
      throw new Error(
        `Configured ERST_PKCS11_TOKEN_LABEL (${opts.tokenLabel}) did not match any tokens. Available tokens: ${available.join(', ')}`
      );
    }
    throw new Error(
      `Configured ERST_PKCS11_TOKEN_LABEL (${opts.tokenLabel}) did not match any tokens. No token labels were available.`
    );
  }

  if (opts.slotIndex) {
    return opts.slots[Number(opts.slotIndex)];
  }

  return opts.slots[0];
};

/**
 * PKCS#11-backed signer.
 *
 * Configuration is done via environment variables.
 *
 * Required env (typical):
 * - ERST_PKCS11_MODULE  : path to PKCS#11 module (e.g. /usr/lib/softhsm/libsofthsm2.so)
 * - ERST_PKCS11_TOKEN_LABEL or ERST_PKCS11_SLOT
 * - ERST_PKCS11_PIN
 * - ERST_PKCS11_KEY_LABEL or ERST_PKCS11_KEY_ID or ERST_PKCS11_PIV_SLOT
 *
 * Notes:
 * - This implementation uses the optional dependency `pkcs11js`.
 * - We intentionally do not hardcode any secrets.
 */
export class Pkcs11Ed25519Signer implements AuditSigner {
  private readonly cfg = {
    module: process.env.ERST_PKCS11_MODULE,
    tokenLabel: process.env.ERST_PKCS11_TOKEN_LABEL,
    slot: process.env.ERST_PKCS11_SLOT,
    pin: process.env.ERST_PKCS11_PIN,
    keyLabel: process.env.ERST_PKCS11_KEY_LABEL,
    keyIdHex: process.env.ERST_PKCS11_KEY_ID,
    pivSlot: process.env.ERST_PKCS11_PIV_SLOT,
    publicKeyPem: process.env.ERST_PKCS11_PUBLIC_KEY_PEM,
  };

  private pkcs11: any | undefined;

  constructor() {
    // Lazy-load so users/tests without PKCS#11 don't break unless provider is selected.
    try {
      this.pkcs11 = lazyRequire('pkcs11js');
    } catch {
      throw new Error(
        'pkcs11 provider selected but optional dependency `pkcs11js` is not installed (add it to dependencies to enable PKCS#11)'
      );
    }

    if (!this.cfg.module) {
      throw new Error('pkcs11 provider selected but ERST_PKCS11_MODULE is not set');
    }
    if (!this.cfg.pin) {
      throw new Error('pkcs11 provider selected but ERST_PKCS11_PIN is not set');
    }
    if (!this.cfg.keyLabel && !this.cfg.keyIdHex && !this.cfg.pivSlot) {
      throw new Error(
        'pkcs11 provider selected but neither ERST_PKCS11_KEY_LABEL, ERST_PKCS11_KEY_ID, nor ERST_PKCS11_PIV_SLOT is set'
      );
    }
  }

  async public_key(): Promise<PublicKey> {
    // We allow providing the public key via env/config to avoid a brittle, token-specific extraction flow.
    // If not provided, we attempt to read it from the token.
    if (this.cfg.publicKeyPem) return this.cfg.publicKeyPem;

    const msg =
      'pkcs11 public key retrieval is not configured. Set ERST_PKCS11_PUBLIC_KEY_PEM to a SPKI PEM public key.';
    throw new Error(msg);
  }

  async sign(payload: Uint8Array): Promise<Signature> {
    // Guard against HSM abuse during loops
    await HsmRateLimiter.checkAndRecordCall();

    // Minimal skeleton that surfaces errors clearly.
    // Implementing full PKCS#11 key discovery + Ed25519 mechanisms depends on token capabilities.
    // We keep this as a real provider module but require ERST_PKCS11_PUBLIC_KEY_PEM for verification.

    const pkcs11 = this.pkcs11;
    if (!pkcs11) throw new Error('pkcs11 internal error: module not loaded');

    // This is a deliberately conservative implementation: open session, login, locate key, sign.
    // If your token does not support Ed25519 sign via CKM_EDDSA, it will error with a clear message.

    const lib = new pkcs11.PKCS11();
    try {
      try {
        lib.load(this.cfg.module);
      } catch (e) {
        const msg = e instanceof Error ? e.message : String(e);
        throw new Error(`Failed to load PKCS#11 module at '${this.cfg.module}': ${msg}. Check that the library exists and is accessible.`);
      }

      try {
        lib.C_Initialize();
      } catch (e) {
        const msg = e instanceof Error ? e.message : String(e);
        const lowerMsg = msg.toLowerCase();
        
        let context = '';
        if (lowerMsg.includes('cryptoki already initialized') || lowerMsg.includes('0x191')) {
          context = 'Library already initialized (CKR_CRYPTOKI_ALREADY_INITIALIZED)';
        } else if (lowerMsg.includes('library lock') || lowerMsg.includes('0x30')) {
          context = 'Library lock error (CKR_CANT_LOCK). The PKCS#11 library may be in use by another process.';
        } else if (lowerMsg.includes('token not present') || lowerMsg.includes('0xe0')) {
          context = 'Token not present (CKR_TOKEN_NOT_PRESENT). Ensure the HSM/token is connected.';
        } else if (lowerMsg.includes('device error') || lowerMsg.includes('0x30')) {
          context = 'Device error (CKR_DEVICE_ERROR). Check HSM/token hardware connection.';
        } else if (lowerMsg.includes('general error') || lowerMsg.includes('0x5')) {
          context = 'General error (CKR_GENERAL_ERROR). The PKCS#11 module failed to initialize.';
        } else {
          context = 'PKCS#11 initialization failed';
        }
        
        throw new Error(`${context}: ${msg}`);
      }

      const slots = lib.C_GetSlotList(true);
      if (!slots || slots.length === 0) {
        throw new Error('No PKCS#11 slots with tokens found. Ensure the HSM/token is connected and recognized by the PKCS#11 module.');
      }

      // Choose slot
      const slot = resolvePkcs11Slot({
        slots,
        slotIndex: this.cfg.slot,
        tokenLabel: this.cfg.tokenLabel,
        getTokenInfo: (slotId) => lib.C_GetTokenInfo(slotId),
      });
      if (slot === undefined) {
        throw new Error(`Configured ERST_PKCS11_SLOT (${this.cfg.slot}) did not resolve to a valid slot. Available slots: ${slots.length}`);
      }

      let session;
      try {
        session = lib.C_OpenSession(slot, pkcs11.CKF_SERIAL_SESSION | pkcs11.CKF_RW_SESSION);
      } catch (e) {
        const msg = e instanceof Error ? e.message : String(e);
        throw new Error(`Failed to open session on slot ${slot}: ${msg}. The token may be locked or unavailable.`);
      }

      try {
        try {
          lib.C_Login(session, 1 /* CKU_USER */, this.cfg.pin);
        } catch (e) {
          const msg = e instanceof Error ? e.message : String(e);
          const lowerMsg = msg.toLowerCase();
          
          let context = '';
          if (lowerMsg.includes('pin incorrect') || lowerMsg.includes('0xa0')) {
            context = 'Wrong PIN (CKR_PIN_INCORRECT)';
          } else if (lowerMsg.includes('pin locked') || lowerMsg.includes('0xa4')) {
            context = 'PIN locked (CKR_PIN_LOCKED). The token may be locked due to too many failed attempts.';
          } else if (lowerMsg.includes('user already logged in') || lowerMsg.includes('0x100')) {
            context = 'User already logged in (CKR_USER_ALREADY_LOGGED_IN)';
          } else if (lowerMsg.includes('session closed') || lowerMsg.includes('0x90')) {
            context = 'Session closed (CKR_SESSION_CLOSED)';
          } else if (lowerMsg.includes('token not present') || lowerMsg.includes('0xe0')) {
            context = 'Token not present (CKR_TOKEN_NOT_PRESENT)';
          } else {
            context = 'Login failed';
          }
          
          throw new Error(`${context}: ${msg}`);
        }

        // Locate private key by label or id
        const template: any[] = [{ type: pkcs11.CKA_CLASS, value: pkcs11.CKO_PRIVATE_KEY }];
        if (this.cfg.keyLabel) template.push({ type: pkcs11.CKA_LABEL, value: this.cfg.keyLabel });
        const keyIdHex = resolvePkcs11KeyIdHex(this.cfg);
        if (keyIdHex) template.push({ type: pkcs11.CKA_ID, value: Buffer.from(keyIdHex, 'hex') });

        lib.C_FindObjectsInit(session, template);
        const keys = lib.C_FindObjects(session, 1);
        lib.C_FindObjectsFinal(session);

        const key = keys?.[0];
        if (!key) throw new Error('private key not found on token (check ERST_PKCS11_KEY_LABEL / ERST_PKCS11_KEY_ID)');

        // Attempt EdDSA sign (CKM_EDDSA). Some tokens use different mechanisms.
        const mechanism = { mechanism: (pkcs11 as any).CKM_EDDSA ?? 0x00001050 };

        try {
          lib.C_SignInit(session, mechanism, key);
          const sig = lib.C_Sign(session, Buffer.from(payload));
          return Buffer.from(sig);
        } catch (e) {
          const msg = e instanceof Error ? e.message : String(e);
          throw new Error(`pkcs11 signing failed: ${msg}`);
        }
      } finally {
        try {
          lib.C_CloseSession(session);
        } catch {
          // ignore
        }
      }
    } catch (e) {
      const msg = e instanceof Error ? e.message : String(e);
      throw new Error(`pkcs11 error: ${msg}`);
    } finally {
      try {
        lib.C_Finalize();
      } catch {
        // ignore
      }
    }
  }
}
