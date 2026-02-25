// Copyright (c) Hintents Authors.
// SPDX-License-Identifier: Apache-2.0

import type { AuditSigner } from './types';
import { SoftwareEd25519Signer } from './softwareSigner';
import { Pkcs11Ed25519Signer } from './pkcs11Signer';
import { KmsSigner } from './kmsSigner';

export type SigningProvider = 'software' | 'pkcs11' | 'kms';

export interface CreateAuditSignerOpts {
  /** Signing provider to use. Defaults to 'software'. */
  hsmProvider?: string;
  /** Ed25519 PKCS#8 PEM private key. Required when provider is 'software'. */
  softwarePrivateKeyPem?: string;
  /**
   * KMS key ID or ARN. May also be supplied via ERST_KMS_KEY_ID.
   * Only used when provider is 'kms'.
   */
  kmsKeyId?: string;
  /**
   * KMS signing algorithm. Defaults to ECDSA_SHA_256.
   * May also be supplied via ERST_KMS_SIGNING_ALGORITHM.
   * Only used when provider is 'kms'.
   */
  kmsSigningAlgorithm?: string;
}

export function createAuditSigner(opts: CreateAuditSignerOpts): AuditSigner {
  const provider = (opts.hsmProvider?.toLowerCase() ?? 'software') as SigningProvider;

  if (provider === 'pkcs11') {
    return new Pkcs11Ed25519Signer();
  }

  if (provider === 'kms') {
    return new KmsSigner({
      keyId: opts.kmsKeyId,
      signingAlgorithm: opts.kmsSigningAlgorithm,
    });
  }

  if (provider === 'software') {
    if (!opts.softwarePrivateKeyPem) {
      throw new Error('software signing selected but no private key was provided');
    }
    return new SoftwareEd25519Signer(opts.softwarePrivateKeyPem);
  }

  throw new Error(`unknown signing provider: "${provider}". Valid options: software, pkcs11, kms`);
}
