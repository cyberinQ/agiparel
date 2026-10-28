// Author: Sunil Jain, 2026-10-28
// Excel sheets prefill editable cutting inputs, never submit without operator review.
(() => {
  'use strict';
  for (const input of document.querySelectorAll('[data-cut-sheet]')) {
    input.addEventListener('change', async () => {
      if (!input.files.length) return;
      const textarea = input.closest('label')?.querySelector(`textarea[name="${input.dataset.cutTarget}"]`);
      if (!textarea) return;
      const statusId = input.dataset.cutSheet === 'issue_allocations' ? 'spread-message' : 'cut-message';
      const status = document.getElementById(statusId);
      const form = new FormData();
      form.append('kind', input.dataset.cutSheet);
      form.append('file', input.files[0]);
      input.disabled = true;
      if (status) status.textContent = 'Reading your spreadsheet…';
      try {
        const res = await fetch('/api/v1/tabular-preview', { method: 'POST', credentials: 'same-origin', body: form });
        if (!res.ok) throw new Error((await res.text()).trim().slice(0, 240));
        const data = await res.json();
        if (!Array.isArray(data.lines) || !data.lines.length) throw new Error('No rows found.');
        textarea.value = data.lines.join('\n');
        textarea.dispatchEvent(new Event('input', { bubbles: true }));
        if (status) status.textContent = `${data.lines.length} row(s) copied from ${data.filename}. Review them before saving.`;
      } catch (err) {
        if (status) status.textContent = `Spreadsheet not loaded: ${err.message}`;
      } finally {
        input.disabled = false;
        input.value = '';
      }
    });
  }
})();
