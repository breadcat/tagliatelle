  function openTagDetailsFromHash() {
    if (!window.location.hash) return;
    const target = document.getElementById(window.location.hash.slice(1));
    if (!target) return;
    const details = target.closest('details');
    if (details) {
      details.open = true;
      target.scrollIntoView();
    }
  }
  window.addEventListener('DOMContentLoaded', openTagDetailsFromHash);
  window.addEventListener('hashchange', openTagDetailsFromHash);