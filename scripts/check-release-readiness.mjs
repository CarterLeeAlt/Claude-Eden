// Inspect presence only. Never print or persist credential values.
const required = [];
const provider = process.env.EDEN_WINDOWS_SIGNING_PROVIDER || 'certificate-store';
if (provider === 'esigner') {
  required.push('EDEN_ESIGNER_USERNAME', 'EDEN_ESIGNER_PASSWORD', 'EDEN_ESIGNER_CREDENTIAL_ID', 'EDEN_ESIGNER_CERTIFICATE_SERIAL', 'EDEN_ESIGNER_TOTP_SECRET');
} else if (provider === 'certificate-store') {
  required.push('EDEN_WINDOWS_PFX_BASE64', 'EDEN_WINDOWS_PFX_PASSWORD');
} else {
  throw new Error('Unknown EDEN_WINDOWS_SIGNING_PROVIDER; use esigner or certificate-store.');
}
const missing = required.filter(name => !process.env[name]?.trim());
if (missing.length) {
  console.error(`Configure these eden-release environment secrets/variables before requesting a release:\n${missing.join('\n')}`);
  process.exitCode = 1;
} else {
  if (provider === 'esigner' && (!/^[0-9a-f-]{36}$/i.test(process.env.EDEN_ESIGNER_CREDENTIAL_ID) ||
    !/^[0-9a-f]{32,40}$/i.test(process.env.EDEN_ESIGNER_CERTIFICATE_SERIAL))) {
    throw new Error('Invalid eSigner credential ID or expected certificate serial.');
  }
  console.log('Release configuration is present. Platform jobs will verify the credentials, signatures and corresponding sources.');
}
