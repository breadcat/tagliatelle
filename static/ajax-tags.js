
function initAjaxTagForms() {
  document.querySelectorAll('.ajax-tag-form').forEach((form) => {
    if (form.dataset.ajaxBound) return; // avoid double-binding after a swap
    form.dataset.ajaxBound = 'true';
    form.addEventListener('submit', handleAjaxTagSubmit);
  });
}

async function handleAjaxTagSubmit(e) {
  e.preventDefault();
  const form = e.target;
  const submitButton = form.querySelector('button[type="submit"]');
  if (submitButton) submitButton.disabled = true;

  const action = form.getAttribute('action') || window.location.pathname;

  try {
    const response = await fetch(action, {
      method: 'POST',
      body: new FormData(form),
    });

    if (!response.ok) {
      throw new Error('Request failed with status ' + response.status);
    }

    const html = await response.text();
    const doc = new DOMParser().parseFromString(html, 'text/html');

    // Swap the tag list
    const newTagsList = doc.getElementById('tags-list');
    const currentTagsList = document.getElementById('tags-list');
    if (newTagsList && currentTagsList) {
      currentTagsList.innerHTML = newTagsList.innerHTML;
    }

    // Swap the category datalist (new categories may have been created)
    const newCategories = doc.getElementById('categories');
    const currentCategories = document.getElementById('categories');
    if (newCategories && currentCategories) {
      currentCategories.innerHTML = newCategories.innerHTML;
    }

    // Show success/error message, pulled from the redirected URL's query params
    const finalUrl = new URL(response.url, window.location.origin);
    const statusEl = document.getElementById('tag-status');
    const success = finalUrl.searchParams.get('success');
    const error = finalUrl.searchParams.get('error');
    if (statusEl) {
      if (error) {
        statusEl.textContent = error;
        statusEl.className = 'tag-status-error';
      } else if (success) {
        statusEl.textContent = success;
        statusEl.className = 'tag-status-success';
      } else {
        statusEl.textContent = '';
        statusEl.className = '';
      }
      if (success || error) {
        setTimeout(() => {
          statusEl.textContent = '';
          statusEl.className = '';
        }, 4000);
      }
    }

    // Clear the "Add Tag" inputs after a successful add
    if (!error) {
      const catInput = form.querySelector('input[name="category"]');
      const valInput = form.querySelector('input[name="value"]');
      if (catInput && form.querySelector('button[type="submit"]').textContent.includes('Add')) {
        catInput.value = '';
        valInput.value = '';
      }
    }

    // Re-bind the delete buttons that just got swapped in
    initAjaxTagForms();
  } catch (err) {
    console.error('Tag update failed:', err);
    const statusEl = document.getElementById('tag-status');
    if (statusEl) {
      statusEl.textContent = 'Failed to update tag. Please try again.';
      statusEl.className = 'tag-status-error';
    }
  } finally {
    if (submitButton) submitButton.disabled = false;
  }
}

document.addEventListener('DOMContentLoaded', initAjaxTagForms);
