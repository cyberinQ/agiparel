// Author: Sunil Jain, 2026-10-29.
// Keep historical static-contract fixtures available from one immutable archive.
'use strict';
const fs = require('node:fs');
const path = require('node:path');
const cp = require('node:child_process');
const archive = path.resolve(__dirname, 'fixtures/v41_legacy_migrations.tar.gz');
const proxy = Object.create(fs);
proxy.readFileSync = (file, options) => {
  if (typeof file === 'string' && (/(?:^|[/\\])services[/\\]core-api[/\\]migrations[/\\]0\d{3}_[^/\\]+\.sql$/.test(file) || /(?:^|[/\\])scripts[/\\]install_release_v(?:3[5-9]|4[01])\.sh$/.test(file))) {
    const name = path.basename(file);
    const raw = cp.execFileSync('tar', ['-xOzf', archive, name], {maxBuffer: 4 * 1024 * 1024});
    const encoding = typeof options === 'string' ? options : options?.encoding;
    return encoding ? raw.toString(encoding) : raw;
  }
  return fs.readFileSync(file, options);
};
module.exports = proxy;
